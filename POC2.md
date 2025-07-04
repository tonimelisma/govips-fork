# **Govips Fusion Pipeline: Proof-of-Concept Plan**

## **1\. Goal & Philosophy**

**Goal:** To validate that we can construct and execute a multi-step libvips processing pipeline from Go in a way that enables **operation fusion**. This is a low-level C-interop proof-of-concept, not a Go API design exercise.  
**Philosophy:** The libvips documentation makes it clear that its performance comes from its ability to see a full dependency graph of operations before computing pixels. It can then "fuse" these operations, streaming pixels through the entire chain in a single pass, minimizing memory usage and maximizing CPU cache efficiency.  
Our task is to replicate this C-level chaining pattern in Go. We will manually build a two-step pipeline (resize \-\> sharpen) by creating two VipsOperation objects and connecting the output of the first to the input of the second. We will then trigger computation on the *final* operation and verify that libvips correctly executes the entire chain.  
This POC will prove we understand the libvips execution model and can correctly manage the associated object lifecycles and memory from Go.

## **2\. The libvips Fusion Mechanism**

Based on the documentation, fusion is achieved by building a graph of dependencies. The key steps are:

1. **Instantiate Operations:** Create all necessary VipsOperation objects (e.g., one for resize, one for sharpen).  
2. **Connect the Graph:** Set the input arguments for each operation. Crucially, the in argument for the sharpen operation will be set to the *out argument* of the resize operation. This creates the dependency link. At this stage, no pixels are processed; we are just building a plan.  
3. **Trigger Execution:** Call vips\_cache\_operation\_build() on the **final** VipsOperation in the chain (sharpen). This tells libvips: "I need the output of this operation." libvips then works backward through the dependency graph it now understands (sharpen needs resize, resize needs the source image) and executes the entire pipeline efficiently.

## **3\. Scope & Key C APIs**

This POC is strictly additive and will be contained within a new test file, vips/fusion\_poc\_test.go.  
**In Scope:**

* A single Go function, pocFusedResizeAndSharpen(), that directly uses cgo to implement the fusion mechanism described above.  
* A small C helper library for GValue manipulation to handle VipsImage\*, gdouble, and gint types.  
* A test that executes the function and verifies the output against the existing sequential method.

**Key C APIs to be Used:**

* vips\_operation\_new(nickname): To create operation objects.  
* g\_object\_set\_property(op, name, gvalue): To set input arguments like the source image, scale factor, and sigma.  
* g\_object\_get\_property(op, name, gvalue): To get a reference to an *output* argument of one operation so it can be used as an *input* for the next.  
* vips\_cache\_operation\_build(op): To trigger the execution of the final operation in the pipeline.  
* g\_object\_ref() / g\_object\_unref(): For correct memory management of all VipsOperation and VipsImage objects.

## **4\. Detailed Implementation Plan**

### **Step 1: Create the GValue Helper Library**

This is a prerequisite. We need a small, robust set of C functions to handle the GValue type, which is the universal container for all operation arguments.  
**Create vips/gvalue\_helpers.h and vips/gvalue\_helpers.c:**  
// In vips/gvalue\_helpers.h  
\#ifndef GOVIPS\_GVALUE\_HELPERS\_H  
\#define GOVIPS\_GVALUE\_HELPERS\_H  
\#include \<vips/vips.h\>

// Functions to SET a GValue from a Go type  
void govips\_g\_value\_set\_image(GValue \*gvalue, VipsImage \*image);  
void govips\_g\_value\_set\_double(GValue \*gvalue, double d);  
void govips\_g\_value\_set\_int(GValue \*gvalue, int i);  
void govips\_g\_value\_set\_string(GValue \*gvalue, const char\* s);

// Function to GET a VipsImage from a GValue  
VipsImage\* govips\_g\_value\_get\_image(GValue \*gvalue);

\#endif

*(The .c file will contain the implementations using g\_value\_init, g\_value\_set\_\*, etc.)*

### **Step 2: Implement the Fused Pipeline Function**

This is the core of the POC. We will create a single Go function that builds and executes the two-step pipeline. The comments explain each C call's purpose.  
**In vips/fusion\_poc\_test.go:**  
package vips

/\*  
\#cgo pkg-config: vips  
\#include "gvalue\_helpers.c"  
\*/  
import "C"  
import (  
    "testing"  
    "unsafe"  
    // ...  
)

// pocFusedResizeAndSharpen demonstrates building a fused operation pipeline.  
func pocFusedResizeAndSharpen(inImage \*ImageRef, scale float64, sigma float64) (\*ImageRef, error) {  
    // \--- Operation 1: RESIZE \---  
    // 1a. Create the 'resize' operation object.  
    resizeNickname := C.CString("resize")  
    defer C.free(unsafe.Pointer(resizeNickname))  
    resizeOp := C.vips\_operation\_new(resizeNickname)  
    if resizeOp \== nil {  
        return nil, anError("failed to create resize op")  
    }  
    // We must unref this operation object when we're done with it.  
    defer C.g\_object\_unref(unsafe.Pointer(resizeOp))

    // 1b. Set the input arguments for 'resize'.  
    // Set the primary input image.  
    var gvalueIn C.GValue  
    C.govips\_g\_value\_set\_image(\&gvalueIn, inImage.image)  
    C.g\_object\_set\_property(  
        (\*C.GObject)(unsafe.Pointer(resizeOp)), C.CString("in"), \&gvalueIn)  
    C.g\_value\_unset(\&gvalueIn)

    // Set the 'scale' argument.  
    var gvalueScale C.GValue  
    C.govips\_g\_value\_set\_double(\&gvalueScale, C.double(scale))  
    C.g\_object\_set\_property(  
        (\*C.GObject)(unsafe.Pointer(resizeOp)), C.CString("scale"), \&gvalueScale)  
    C.g\_value\_unset(\&gvalueScale)

    // \--- Operation 2: SHARPEN \---  
    // 2a. Create the 'sharpen' operation object.  
    sharpenNickname := C.CString("sharpen")  
    defer C.free(unsafe.Pointer(sharpenNickname))  
    sharpenOp := C.vips\_operation\_new(sharpenNickname)  
    if sharpenOp \== nil {  
        return nil, anError("failed to create sharpen op")  
    }  
    defer C.g\_object\_unref(unsafe.Pointer(sharpenOp))

    // 2b. Set the input arguments for 'sharpen'.  
    // THIS IS THE FUSION LINK: Set the 'in' of sharpen to the 'out' of resize.  
    var gvalueResizeOut C.GValue  
    // Get the property 'out' from the resize operation. This doesn't run the  
    // operation, it just gets a reference to its eventual output.  
    C.g\_object\_get\_property(  
        (\*C.GObject)(unsafe.Pointer(resizeOp)), C.CString("out"), \&gvalueResizeOut)  
    // Now set that output as the input for the sharpen operation.  
    C.g\_object\_set\_property(  
        (\*C.GObject)(unsafe.Pointer(sharpenOp)), C.CString("in"), \&gvalueResizeOut)  
    C.g\_value\_unset(\&gvalueResizeOut)

    // Set the 'sigma' argument for sharpen.  
    var gvalueSigma C.GValue  
    C.govips\_g\_value\_set\_double(\&gvalueSigma, C.double(sigma))  
    C.g\_object\_set\_property(  
        (\*C.GObject)(unsafe.Pointer(sharpenOp)), C.CString("sigma"), \&gvalueSigma)  
    C.g\_value\_unset(\&gvalueSigma)

    // \--- EXECUTION \---  
    // 3\. Build the FINAL operation in the chain ('sharpen').  
    // This triggers libvips to compute the entire dependency graph.  
    builtOp := C.vips\_cache\_operation\_build(sharpenOp)  
    if builtOp \== nil {  
        return nil, handleVipsError()  
    }  
    // The builtOp is a new object we are responsible for.  
    defer C.g\_object\_unref(unsafe.Pointer(builtOp))

    // 4\. Get the final output image from the built operation.  
    var gvalueFinalOut C.GValue  
    C.g\_object\_get\_property(  
        (\*C.GObject)(unsafe.Pointer(builtOp)), C.CString("out"), \&gvalueFinalOut)  
    finalImagePtr := C.govips\_g\_value\_get\_image(\&gvalueFinalOut)  
    C.g\_value\_unset(\&gvalueFinalOut)

    // The image from get\_property is not ref'd. We must add a reference for our Go  
    // ImageRef to hold, which will be managed by the Go GC and finalizer.  
    C.g\_object\_ref(unsafe.Pointer(finalImagePtr))

    return newImageRef(finalImagePtr, inImage.format, inImage.originalFormat, nil), nil  
}

### **Step 3: Write the Test Case**

The test will execute our fused pipeline and compare its output to the output of running the existing govips methods sequentially. If they are identical, the POC is a success.  
**In vips/fusion\_poc\_test.go:**  
func TestFusionPipelinePOC(t \*testing.T) {  
    // 1\. Get the expected result using the existing sequential API.  
    expectedImg, \_ := LoadImageFromFile("../resources/jpg-24bit-icc-iec.jpg")  
    defer expectedImg.Close()  
    \_ \= expectedImg.Resize(0.5, KernelLanczos3)  
    \_ \= expectedImg.Sharpen(1.5, 0, 0\) // Using simple sharpen params for test

    // 2\. Get the actual result using our new fused pipeline function.  
    inImg, \_ := LoadImageFromFile("../resources/jpg-24bit-icc-iec.jpg")  
    defer inImg.Close()  
    actualImg, err := pocFusedResizeAndSharpen(inImg, 0.5, 1.5)  
    require.NoError(t, err)  
    require.NotNil(t, actualImg)  
    defer actualImg.Close()

    // 3\. Verify the results are identical.  
    // The most reliable way is to find the absolute difference and check its average.  
    diffImg, err := actualImg.Subtract(expectedImg)  
    require.NoError(t, err)  
    defer diffImg.Close()

    avg, err := diffImg.Average()  
    require.NoError(t, err)  
    assert.Less(t, avg, 0.001, "images should be identical")  
    t.Log("Fusion pipeline POC successful.")  
}

## **5\. Success Criteria**

The POC will be considered successful when:

1. The TestFusionPipelinePOC test passes, proving our fused pipeline produces the same result as the sequential one.  
2. The test runs without memory leaks, confirmed via vips\_object\_print\_all() or other memory debugging tools.  
3. We have a clear, working Go function that demonstrates the correct pattern for chaining VipsOperation objects. This proves we have the foundational knowledge required to build a robust code generator.