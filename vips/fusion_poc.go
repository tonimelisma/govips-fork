package vips

/*
#cgo pkg-config: vips
#include "gvalue_helpers.h"
*/
import "C"
import (
	"unsafe"
)

// pocFusedResizeAndSharpen demonstrates building a fused operation pipeline.
func pocFusedResizeAndSharpen(inImage *ImageRef, scale float64, sigma float64) (*ImageRef, error) {
	// --- Operation 1: RESIZE ---
	// 1a. Create the 'resize' operation object.
	resizeNickname := C.CString("resize")
	defer C.free(unsafe.Pointer(resizeNickname))
	resizeOp := C.vips_operation_new(resizeNickname)
	if resizeOp == nil {
		return nil, handleVipsError()
	}
	// We must unref this operation object when we're done with it.
	defer C.g_object_unref(C.gpointer(resizeOp))

	// 1b. Set the input arguments for 'resize'.
	// Set the primary input image.
	var gvalueIn C.GValue
	C.govips_g_value_set_image(&gvalueIn, inImage.image)
	propIn := C.CString("in")
	defer C.free(unsafe.Pointer(propIn))
	C.g_object_set_property(
		(*C.GObject)(unsafe.Pointer(resizeOp)), propIn, &gvalueIn)
	C.g_value_unset(&gvalueIn)

	// Set the 'scale' argument.
	var gvalueScale C.GValue
	C.govips_g_value_set_double(&gvalueScale, C.double(scale))
	propScale := C.CString("scale")
	defer C.free(unsafe.Pointer(propScale))
	C.g_object_set_property(
		(*C.GObject)(unsafe.Pointer(resizeOp)), propScale, &gvalueScale)
	C.g_value_unset(&gvalueScale)

	// --- Operation 2: SHARPEN ---
	// 2a. Create the 'sharpen' operation object.
	sharpenNickname := C.CString("sharpen")
	defer C.free(unsafe.Pointer(sharpenNickname))
	sharpenOp := C.vips_operation_new(sharpenNickname)
	if sharpenOp == nil {
		return nil, handleVipsError()
	}
	defer C.g_object_unref(C.gpointer(sharpenOp))

	// 2b. Set the input arguments for 'sharpen'.
	// THIS IS THE FUSION LINK: Set the 'in' of sharpen to the 'out' of resize.
	var gvalueResizeOut C.GValue
	propOut := C.CString("out")
	defer C.free(unsafe.Pointer(propOut))
	// Get the property 'out' from the resize operation. This doesn't run the
	// operation, it just gets a reference to its eventual output.
	C.g_object_get_property(
		(*C.GObject)(unsafe.Pointer(resizeOp)), propOut, &gvalueResizeOut)
	// Now set that output as the input for the sharpen operation.
	C.g_object_set_property(
		(*C.GObject)(unsafe.Pointer(sharpenOp)), propIn, &gvalueResizeOut)
	C.g_value_unset(&gvalueResizeOut)

	// Set the 'sigma' argument for sharpen.
	var gvalueSigma C.GValue
	C.govips_g_value_set_double(&gvalueSigma, C.double(sigma))
	propSigma := C.CString("sigma")
	defer C.free(unsafe.Pointer(propSigma))
	C.g_object_set_property(
		(*C.GObject)(unsafe.Pointer(sharpenOp)), propSigma, &gvalueSigma)
	C.g_value_unset(&gvalueSigma)

	// --- EXECUTION ---
	// 3. Build the FINAL operation in the chain ('sharpen').
	// This triggers libvips to compute the entire dependency graph.
	builtOp := C.vips_cache_operation_build(sharpenOp)
	if builtOp == nil {
		return nil, handleVipsError()
	}
	// The builtOp is a new object we are responsible for.
	defer C.g_object_unref(C.gpointer(builtOp))

	// 4. Get the final output image from the built operation.
	var gvalueFinalOut C.GValue
	C.g_object_get_property(
		(*C.GObject)(unsafe.Pointer(builtOp)), propOut, &gvalueFinalOut)
	finalImagePtr := C.govips_g_value_get_image(&gvalueFinalOut)
	C.g_value_unset(&gvalueFinalOut)

	// The image from get_property is not ref'd. We must add a reference for our Go
	// ImageRef to hold, which will be managed by the Go GC and finalizer.
	C.g_object_ref(C.gpointer(finalImagePtr))

	return newImageRef(finalImagePtr, inImage.format, inImage.originalFormat, nil), nil
}
