package elasticsearch

import (
	"testing"
)

func TestHeaderGenEmptyToken(t *testing.T) {
	tokenFile := "../../test/files/emptyToken"

	_, ok := readSAToken(tokenFile)

	if ok {
		t.Errorf("Expected to be unable to read file [%s] due to empty file but could", tokenFile)
	}
}

func TestHeaderGenWithToken(t *testing.T) {
	tokenFile := "../../test/files/testToken"

	expected := "test\n"

	actual, ok := readSAToken(tokenFile)

	if !ok {
		t.Errorf("Expected to be able to read file [%s] but couldn't", tokenFile)
	} else {
		if actual != expected {
			t.Errorf("Expected %q but got %q", expected, actual)
		}
	}
}

func TestHeaderGenWithNoToken(t *testing.T) {
	tokenFile := "../../test/files/errorToken"

	_, ok := readSAToken(tokenFile)

	if ok {
		t.Errorf("Expected to be unable to read file [%s]", tokenFile)
	}
}
