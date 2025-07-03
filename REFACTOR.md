# **Govips: A New Architecture**

**A Maintainer's Guide to a Future-Proof govips**

## **1\. Vision & Philosophy**

The current govips implementation is a success. It provides a functional, idiomatic Go interface for libvips. However, its architecture is built on manual labor. Every C function is wrapped by hand, twice: once in a C shim and again in Go. This creates a significant maintenance burden and a constant risk of the Go API falling out of sync with the underlying C library.  
**The new philosophy is simple: The machine should do the work.**  
We will build a system that introspects the libvips C library and automatically generates the Go bindings. This will ensure that govips is always a complete, up-to-date, and type-safe reflection of libvips's capabilities. The result is a library that is easier to maintain, more comprehensive for users, and fundamentally more robust.  
The end-user experience will not change. The code generation happens at development time; the user's go get and go build experience remains seamless. They simply get a better, more complete library.

## **2\. The Core Problem: The Manual Treadmill**

The current architecture is defined by files like arithmetic.c and arithmetic.go.  
**In arithmetic.c:**  
int add(VipsImage \*left, VipsImage \*right, VipsImage \*\*out) {  
  return vips\_add(left, right, out, NULL);  
}

**In arithmetic.go:**  
func vipsAdd(left \*C.VipsImage, right \*C.VipsImage) (\*C.VipsImage, error) {  
	// ...  
	if err := C.add(left, right, \&out); err \!= 0 {  
		return nil, handleImageError(out)  
	}  
	return out, nil  
}

This pattern has two fundamental weaknesses:

1. **High Maintenance:** Adding a new libvips function requires editing three places: the C header, the C implementation, and the Go wrapper. This is tedious and error-prone.  
2. **Incomplete API:** The library can only ever expose the functions that a maintainer has had time to wrap. The long tail of libvips functionality will likely never be implemented this way.

## **3\. The Solution: Introspection & Generation**

We will replace this manual process with a code generator that understands the libvips API. This is the **"Compiled Language"** model described in the libvips documentation, and it is the correct path for Go.  
The core of this new architecture is the VipsOperation object model. Instead of calling vips\_add(), we will programmatically build an "add" operation, set its input arguments, execute it, and retrieve its output arguments.  
The maintainer's workflow will become:

1. Update the system's libvips library.  
2. Run go generate ./... in the govips repository.  
3. Review the changed files and run go test ./....  
4. Commit the newly generated Go files.

## **4\. The Roadmap: A Phased Approach**

This is a significant change, but we can implement it incrementally.

### **Phase 0: The Proof-of-Concept Spike**

Before building the generator, we must prove we can call a single operation using the low-level API. Our target will be vips\_invert.  
**Goal:** Create a standalone Go file that successfully inverts an image using the VipsOperation model, without using any of the existing C shims.  
**Steps:**

1. **Create a C Helper for GValue:** The biggest hurdle is the GValue, a generic container used by libvips. We will need small C helper functions to manage it. Create a gvalue.c file (this will later be part of the generator or a core utility).  
   // gvalue.c  
   \#include \<vips/vips.h\>

   // Helper to set a GValue to a VipsImage  
   void govips\_g\_value\_set\_image(GValue \*gvalue, VipsImage \*image) {  
       g\_value\_init(gvalue, VIPS\_TYPE\_IMAGE);  
       g\_value\_set\_object(gvalue, image);  
   }

   // Helper to get a VipsImage from a GValue  
   VipsImage\* govips\_g\_value\_get\_image(GValue \*gvalue) {  
       return VIPS\_IMAGE(g\_value\_get\_object(gvalue));  
   }

2. **Write the Spike in spike.go:**  
   package vips

   /\*  
   \#cgo pkg-config: vips  
   \#include \<vips/vips.h\>  
   \#include "gvalue.c" // For the spike, we can include it directly  
   \*/  
   import "C"  
   import "unsafe"

   func spikeInvert(in \*ImageRef) (\*ImageRef, error) {  
       // 1\. Create the operation by its nickname  
       opNickname := C.CString("invert")  
       defer C.free(unsafe.Pointer(opNickname))

       op := C.vips\_operation\_new(opNickname)  
       if op \== nil {  
           return nil, anError("could not create 'invert' operation")  
       }  
       defer C.g\_object\_unref(unsafe.Pointer(op))

       // 2\. Set the input arguments  
       // We need a GValue to hold our image  
       var gvalueIn C.GValue  
       C.govips\_g\_value\_set\_image(\&gvalueIn, in.image)

       // Set the "in" property on the operation  
       propIn := C.CString("in")  
       defer C.free(unsafe.Pointer(propIn))  
       C.g\_object\_set\_property((\*C.GObject)(unsafe.Pointer(op)), propIn, \&gvalueIn)  
       C.g\_value\_unset(\&gvalueIn)

       // 3\. Build the operation  
       buildOp := C.vips\_cache\_operation\_build(op)  
       if buildOp \== nil {  
           return nil, anError("could not build 'invert' operation")  
       }  
       defer C.g\_object\_unref(unsafe.Pointer(buildOp))

       // 4\. Get the output arguments  
       var gvalueOut C.GValue  
       C.g\_value\_init(\&gvalueOut, VIPS\_TYPE\_IMAGE)

       propOut := C.CString("out")  
       defer C.free(unsafe.Pointer(propOut))  
       C.g\_object\_get\_property((\*C.GObject)(unsafe.Pointer(buildOp)), propOut, \&gvalueOut)

       outImage := C.govips\_g\_value\_get\_image(\&gvalueOut)  
       C.g\_value\_unset(\&gvalueOut)

       // The output image from get\_property is not ref'd, so we must add one.  
       C.g\_object\_ref(unsafe.Pointer(outImage))

       return newImageRef(outImage, in.format, in.originalFormat, nil), nil  
   }

   *Note: Error handling and other details are simplified for clarity.*

With a working spikeInvert function, the hardest part is done. We've proven the architectural path is sound.

### **Phase 1: Building the Generator**

Now we automate the process from Phase 0\.  
**Goal:** Create a program at cmd/generator/main.go that generates Go wrapper functions for all libvips operations.  
**Steps:**

1. **Define the OperationSpec struct:** This Go struct will hold the information we gather about a libvips operation.  
   // In cmd/generator/main.go  
   type Argument struct {  
       Name      string // e.g. "in", "out", "sigma"  
       GoName    string // e.g. "In", "Out", "Sigma"  
       GoType    string // e.g. "\*ImageRef", "float64"  
       CType     string // e.g. "VIPS\_TYPE\_IMAGE"  
       IsInput   bool  
       IsOutput  bool  
       IsRequired bool  
   }

   type OperationSpec struct {  
       Nickname      string // e.g. "invert"  
       GoName        string // e.g. "Invert"  
       Args          \[\]Argument  
   }

2. **Implement Introspection Logic:** Use cgo to iterate through libvips and populate these structs. This is the core of the generator. You will need C functions to walk through the VipsOperation class and its arguments.  
3. **Create Go Templates:** Use the text/template package to define a template file, cmd/generator/operation.tmpl.  
   // In cmd/generator/operation.tmpl  
   // {{ .GoName }} is the function name, e.g. "Invert"  
   // {{ .Nickname }} is the C nickname, e.g. "invert"

   func (r \*ImageRef) {{ .GoName }}(/\*... args ...\*/) error {  
       // ... generated CGO calls based on the OperationSpec ...  
       // 1\. vips\_operation\_new("{{ .Nickname }}")  
       // 2\. Loop through required input args and set properties  
       // 3\. Loop through optional input args and set properties  
       // 4\. vips\_cache\_operation\_build()  
       // 5\. Loop through output args and get properties  
       // 6\. Update ImageRef with the primary output image  
       // ...  
   }

4. **Run the Generator:** The main function of the generator will orchestrate this:  
   * Call the introspection C functions.  
   * Map C types to Go types.  
   * Loop through all OperationSpecs.  
   * Execute the template for each spec.  
   * Write the result to a file, e.g., gen\_invert.go.

### **Phase 2: Integration**

**Goal:** Integrate the generated code into the main govips package and begin replacing the manual bindings.

1. **Add go:generate:** In a key file like govips.go, add the directive:  
   //go:generate go run ./cmd/generator

2. **Incremental Replacement:** Pick one function, like ImageRef.Invert(). Change its implementation to call the new, generated invert function. Run go test. If it passes, move to the next function. This can be done over multiple pull requests.

### **Phase 3: Finalization**

**Goal:** Remove all legacy manual bindings.

1. **Delete Old Code:** Once all functions are migrated, delete the C shim files (arithmetic.c, header.c, etc.) and their corresponding Go files (arithmetic.go, header.go).  
2. **Cleanup:** Remove any C helper functions that are no longer needed. The cgo preamble in the generated files should be minimal.  
3. **Documentation:** The new functions are self-documenting via their generated code. Ensure the top-level package documentation reflects the new, complete API.

This phased approach mitigates risk and allows the library to remain fully functional throughout the transition. The result will be a govips that is not just a wrapper, but a true, comprehensive Go interface for libvips.