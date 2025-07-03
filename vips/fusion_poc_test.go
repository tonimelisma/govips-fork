package vips

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFusionPipelinePOC(t *testing.T) {
	// 1. Get the expected result using the existing sequential API.
	expectedImg, err := NewImageFromFile("../resources/jpg-24bit-icc-iec.jpg")
	require.NoError(t, err)
	defer expectedImg.Close()
	err = expectedImg.Resize(0.5, KernelLanczos3)
	require.NoError(t, err)
	// Simple sharpen params for test. Note that the C-level sharpen operation
	// does not expose separate m1/m2/x1/y2 params like the high-level one.
	// A sigma of 1.5 is a reasonable sharpen.
	err = expectedImg.Sharpen(1.5, 0, 0)
	require.NoError(t, err)

	// 2. Get the actual result using our new fused pipeline function.
	inImg, err := NewImageFromFile("../resources/jpg-24bit-icc-iec.jpg")
	require.NoError(t, err)
	defer inImg.Close()
	actualImg, err := pocFusedResizeAndSharpen(inImg, 0.5, 1.5)
	require.NoError(t, err)
	require.NotNil(t, actualImg)
	defer actualImg.Close()

	// 3. Verify the results are identical.
	// The most reliable way is to find the absolute difference and check its average.
	diffImg, err := actualImg.Copy()
	require.NoError(t, err)
	defer diffImg.Close()

	err = diffImg.Subtract(expectedImg)
	require.NoError(t, err)

	avg, err := diffImg.Average()
	require.NoError(t, err)
	assert.Less(t, avg, 0.001, "images should be identical")
	t.Log("Fusion pipeline POC successful.")
}
