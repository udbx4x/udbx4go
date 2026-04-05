package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeature_GetAttribute(t *testing.T) {
	f := &Feature{
		ID: 1,
		Attributes: map[string]interface{}{
			"name":       "Beijing",
			"population": 21540000,
		},
	}

	// Get existing attribute
	val, ok := f.GetAttribute("name")
	assert.True(t, ok)
	assert.Equal(t, "Beijing", val)

	// Get another attribute
	val2, ok2 := f.GetAttribute("population")
	assert.True(t, ok2)
	assert.Equal(t, 21540000, val2)

	// Get non-existent attribute
	val3, ok3 := f.GetAttribute("nonexistent")
	assert.False(t, ok3)
	assert.Nil(t, val3)
}

func TestFeature_GetAttribute_NilAttributes(t *testing.T) {
	f := &Feature{
		ID:         1,
		Attributes: nil,
	}

	val, ok := f.GetAttribute("name")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestFeature_SetAttribute(t *testing.T) {
	f := &Feature{
		ID:         1,
		Attributes: nil,
	}

	// Set attribute when nil
	f.SetAttribute("name", "Beijing")
	assert.NotNil(t, f.Attributes)
	assert.Equal(t, "Beijing", f.Attributes["name"])

	// Set another attribute
	f.SetAttribute("population", 21540000)
	assert.Equal(t, 21540000, f.Attributes["population"])

	// Update existing attribute
	f.SetAttribute("name", "Shanghai")
	assert.Equal(t, "Shanghai", f.Attributes["name"])
}

func TestTabularRecord_GetAttribute(t *testing.T) {
	r := &TabularRecord{
		ID: 1,
		Attributes: map[string]interface{}{
			"code": "CN",
			"name": "China",
		},
	}

	// Get existing attribute
	val, ok := r.GetAttribute("code")
	assert.True(t, ok)
	assert.Equal(t, "CN", val)

	// Get non-existent attribute
	val2, ok2 := r.GetAttribute("nonexistent")
	assert.False(t, ok2)
	assert.Nil(t, val2)
}

func TestTabularRecord_GetAttribute_NilAttributes(t *testing.T) {
	r := &TabularRecord{
		ID:         1,
		Attributes: nil,
	}

	val, ok := r.GetAttribute("code")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestTabularRecord_SetAttribute(t *testing.T) {
	r := &TabularRecord{
		ID:         1,
		Attributes: nil,
	}

	// Set attribute when nil
	r.SetAttribute("code", "CN")
	assert.NotNil(t, r.Attributes)
	assert.Equal(t, "CN", r.Attributes["code"])

	// Set another attribute
	r.SetAttribute("name", "China")
	assert.Equal(t, "China", r.Attributes["name"])

	// Update existing attribute
	r.SetAttribute("code", "US")
	assert.Equal(t, "US", r.Attributes["code"])
}
