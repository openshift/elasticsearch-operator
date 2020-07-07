package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/openshift/elasticsearch-operator/pkg/utils"
	"github.com/sirupsen/logrus"
)

const exampleSecretsPath = "/tmp/example-secrets"

func generateCertificates(t *testing.T, namespace, uuid string) error {
	workDir := fmt.Sprintf("%s/%s", exampleSecretsPath, uuid)
	storeName := elasticsearchNameFor(uuid)

	err := os.MkdirAll(workDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create certificate tmp dir: %s", err)
	}

	cmd := exec.Command("./hack/cert_generation.sh", workDir, namespace, storeName)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate certificate for %q: %v\n:%s", storeName, err, string(out))
	}

	return nil
}

func getCertificateContents(name, uuid string) []byte {
	filename := fmt.Sprintf("%s/%s/%s", exampleSecretsPath, uuid, name)
	return utils.GetFileContents(filename)
}

func GetFileContents(filePath string) []byte {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		logrus.Errorf("Unable to read file to get contents: %v", err)
		return nil
	}

	return contents
}
