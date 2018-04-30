package sutils

import (
	"strings"

	"github.com/icrowley/fake"
)

// RandName returns a random username that can be used for databases
func RandName() string {
	tmp := strings.Split(fake.ProductName(), " ")[:2]
	res := strings.ToLower(strings.Join(tmp, "_"))

	if len(res) > 16 {
		res = res[0:16]
	}

	return res
}

// RandDBName returns a random name that can be used as a name for a database
func RandDBName() string {
	return RandName()
}

// RandPassword returns a random password that can be used for databases
func RandPassword() string {
	return RandName()
}
