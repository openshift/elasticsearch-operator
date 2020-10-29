package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func NormalizeJSON(doc string) string {
	doc = strings.TrimSpace(doc)
	data := &map[string]interface{}{}
	if err := json.Unmarshal([]byte(doc), data); err != nil {
		Fail(fmt.Sprintf("Unable to normalize document '%s': %v", doc, err))
	}
	response, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		Fail(fmt.Sprintf("Unable to normalize document '%s': %v", doc, err))
	}
	return string(response)
}

type JSONExpectation struct {
	actual string
}

func ExpectJSON(doc string) *JSONExpectation {
	return &JSONExpectation{actual: doc}
}

func (exp *JSONExpectation) ToEqual(doc string) {
	actual := NormalizeJSON(exp.actual)
	expected := NormalizeJSON(doc)
	if actual != expected {
		fmt.Printf("Actual>:\n%s<\n", actual)
		fmt.Printf("Expected>:\n%s\n<", expected)
		Expect(actual).To(Equal(expected))
	}
}
