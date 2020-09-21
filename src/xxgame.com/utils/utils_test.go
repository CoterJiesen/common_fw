package utils

import (
	"fmt"
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
)

func TestAll(t *testing.T) {
	var e error
	idStr := "5533.322"

	fmt.Println("id str:", idStr)

	fields, e := ConvertServerIDString2Fields(idStr)
	assert.Nil(t, e, "failed")
	fmt.Println("convert to fields:", fields)

	id, e := ConvertServerIDString2Number(idStr)
	assert.Nil(t, e, "failed")
	fmt.Println("convert to number:", id)

	s := ConvertServerIDNumber2String(id)
	assert.Equal(t, s, idStr, "failed")
	fmt.Println("convert back to string:", s)

	f := ConvertServerIDNumber2Fields(id)
	assert.Equal(t, f, fields, "failed")
	fmt.Println("convert back to fields:", f)

}
