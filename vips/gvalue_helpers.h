#ifndef GOVIPS_GVALUE_HELPERS_H
#define GOVIPS_GVALUE_HELPERS_H
#include <vips/vips.h>

// Functions to SET a GValue from a Go type
static inline void govips_g_value_set_image(GValue *gvalue, VipsImage *image) {
    g_value_init(gvalue, VIPS_TYPE_IMAGE);
    g_value_set_object(gvalue, image);
}

static inline void govips_g_value_set_double(GValue *gvalue, double d) {
    g_value_init(gvalue, G_TYPE_DOUBLE);
    g_value_set_double(gvalue, d);
}

static inline void govips_g_value_set_int(GValue *gvalue, int i) {
    g_value_init(gvalue, G_TYPE_INT);
    g_value_set_int(gvalue, i);
}

static inline void govips_g_value_set_string(GValue *gvalue, const char* s) {
    g_value_init(gvalue, G_TYPE_STRING);
    g_value_set_string(gvalue, s);
}

// Function to GET a VipsImage from a GValue
static inline VipsImage* govips_g_value_get_image(GValue *gvalue) {
    return VIPS_IMAGE(g_value_get_object(gvalue));
}

#endif 