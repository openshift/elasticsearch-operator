package elasticsearch

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"testing"
	"time"

	elasticsearch6 "github.com/elastic/go-elasticsearch/v6"
)

func getRootCA() *x509.CertPool {
	certPool := x509.NewCertPool()

	// load cert into []byte
	f := path.Join("./", "admin-ca")
	caPem, err := ioutil.ReadFile(f)
	if err != nil {
		log.Panicf("Unable to read file to get contents %v", err)
		return nil
	}
	log.Printf("ca pem %v", string(caPem))
	certPool.AppendCertsFromPEM(caPem)

	return certPool
}

func getClientCertificates() []tls.Certificate {
	certificate, err := tls.LoadX509KeyPair(
		path.Join("./", "admin-cert"),
		path.Join("./", "admin-key"),
	)
	if err != nil {
		log.Println("erro load key pairs")
		return []tls.Certificate{}
	}
	return []tls.Certificate{
		certificate,
	}
}

func oauthEsClient(esAddr, token, caPath, certPath, keyPath string) (*elasticsearch6.Client, error) {
	es := &elasticsearch6.Client{}
	httpTranport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            getRootCA(),
			// Certificates:       getClientCertificates(),
		},
	}

	header := http.Header{}
	header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	cfg := elasticsearch6.Config{
		Header:    header,
		Addresses: []string{esAddr},
		Transport: httpTranport,
	}
	es, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating the client: %v", err)
	}
	return es, nil
}

func mTLSEsClient(esAddr, caPath, certPath, keyPath string) (*elasticsearch6.Client, error) {
	es := &elasticsearch6.Client{}
	httpTranport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			RootCAs:            getRootCA(),
			Certificates:       getClientCertificates(),
		},
	}

	cfg := elasticsearch6.Config{
		Addresses: []string{esAddr},
		Transport: httpTranport,
	}
	es, err := elasticsearch6.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating the mtls client: %v", err)
	}
	return es, nil
}

func getESClient(esAddr string) (*elasticsearch6.Client, error) {
	es := &elasticsearch6.Client{}

	if esAddr == "" {
		log.Fatalf("es address is empty")
	}
	log.Printf("es address: %s\n", esAddr)

	// Setup es client
	es, err := elasticsearch6.NewClient(elasticsearch6.Config{
		Addresses: []string{esAddr},
	})
	if err != nil {
		log.Fatalf("Error creating the client: %s\n", err)
	}
	return es, nil
}

func TestGetClusterNodeVersion_actual(t *testing.T) {
	esAddr := "http://localhost:9200"

	elasticsearchClient, err := getESClient(esAddr)

	esClient := NewClient("default", "default", *elasticsearchClient)
	log.Printf("testing")
	got, err := esClient.GetClusterNodeVersions()
	log.Println(got)
	log.Println(err)

}
