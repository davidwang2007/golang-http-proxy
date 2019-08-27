package main

import "testing"

func TestCopy(t *testing.T) {

	b1 := []byte{1, 2, 3, 4, 5}
	b2 := []byte{11, 12}

	copy(b1[:1], b2[:2])
	t.Logf("b1 %v, b2 %v\n", b1, b2)

}
