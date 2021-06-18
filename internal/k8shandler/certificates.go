package k8shandler

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	mathRand "math/rand"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/ViaQ/logerr/log"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type certificate struct {
	cert      []byte
	key       []byte
	x509Cert  *x509.Certificate
	privKey   *rsa.PrivateKey
	certMutex sync.Mutex
}

type certCA struct {
	certificate
	serial     *big.Int
	pubKeySHA1 []byte
}

type x509v3Ext struct {
	dns []string
	ips []net.IP
	oid bool
}

// used to store random 32 character alphanum sequence
type sessionSecret struct {
	secret []byte
}

const (
	rsaKeyLength      = 4096
	caCN              = "Logging Signing CA"
	caNotAfterYears   = 5
	compNotAfterYears = 2
	nameTypeDNS       = 2
	nameTypeIP        = 7

	componentKeyName  = "tls.key"
	componentCertName = "tls.crt"
	componentCAName   = "ca-bundle.crt"
	esCAKeyName       = "key"
	esCACertName      = "cert"
	esCASerialName    = "serial"

	esComponentName     = "elasticsearch"
	esComponentKeyName  = "elasticsearch.key"
	esComponentCertName = "elasticsearch.crt"

	esInternalComponentName = "logging-es"
	esInternalKeyName       = "logging-es.key"
	esInternalCertname      = "logging-es.crt"

	esAdminComponentName = "system.admin"
	esAdminKeyName       = "admin-key"
	esAdminCertName      = "admin-cert"
	esAdminCAName        = "admin-ca"

	kibanaComponentName     = "system.logging.kibana"
	kibanaComponentKeyName  = "key"
	kibanaComponentCertName = "cert"
	kibanaComponentCAName   = "ca"

	kibanaInternalComponentName     = "kibana-internal"
	kibanaInternalSessionSecretName = "session-secret"
	kibanaInternalCertName          = "server-cert"
	kibanaInternalKeyName           = "server-key"

	kibanaSecretName = "kibana"

	kibanaSessionSecretLength = 32
)

var (
	allowedRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	bigOne = big.NewInt(1)

	caOrganizationUnit        = []string{"Logging Signing CA"}
	caCountry                 = []string{"io", "openshift"}
	certOrganization          = []string{"OpenShift"}
	componentOrganizationUnit = []string{"Logging"}

	ridRawValue asn1.RawValue = asn1.RawValue{
		FullBytes: []byte{0x88, 0x05, 0x2A, 0x03, 0x04, 0x05, 0x05},
	}

	sanCriticality bool = false

	// The RID.1:1.2.3.4.5.5 x509v3 extension.
	// In ASN.1, "2 5 29 17" is the OID for subjectAltName (SAN)
	// The FullBytes are ASN.1-encoded 1.2.3.4.5.5
	sanIdentifier asn1.ObjectIdentifier = asn1.ObjectIdentifier{2, 5, 29, 17}
)

type CertificateRequest struct {
	ClusterName string
	Namespace   string
	OwnerRef    metav1.OwnerReference
	K8sClient   client.Client

	Extensions map[string]x509v3Ext
}

func NewCertificateRequest(clusterName, namespace string, ownerRef metav1.OwnerReference, client client.Client) *CertificateRequest {
	return &CertificateRequest{
		ClusterName: clusterName,
		Namespace:   namespace,
		OwnerRef:    ownerRef,
		K8sClient:   client,
		Extensions: map[string]x509v3Ext{
			`kibana-internal`: {
				[]string{
					`kibana`,
					`kibana.` + namespace + `.svc`,
				},
				[]net.IP{},
				false,
			},
			`elasticsearch`: {
				[]string{
					`localhost`,
					clusterName + `-cluster`,
					clusterName + `-cluster.` + namespace + `.svc`,
				},
				[]net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
				true,
			},
			`logging-es`: {
				[]string{
					`localhost`,
					clusterName,
					clusterName + `.` + namespace + `.svc`,
				},
				[]net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
				false,
			},
		},
	}
}

func (cr *CertificateRequest) getSigningSecretName() string {
	return fmt.Sprintf("signing-%s", cr.ClusterName)
}

func (cr *CertificateRequest) GenerateComponentCerts(secretName, cn string) {
	secret, err := getSecret(secretName, cr.Namespace, cr.K8sClient)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "unable to get secret")
		return
	}

	componentCert := &certificate{}
	if err := unmarshalCert(secret.Data[componentCertName], secret.Data[componentKeyName], componentCert); err != nil {
		log.Info("Failed to unmarshal cert from secret for component", "error", err)
	}

	err = cr.EnsureCert(cn, componentCert)
	if err != nil {
		log.Error(err, "Unable to generate cert for component")
		return
	}

	ca, err := cr.getCACertBytes()
	if err != nil {
		log.Error(err, "Unable to get CA bytes")
		return
	}

	componentSecretData := map[string][]byte{
		componentKeyName:  componentCert.key,
		componentCertName: componentCert.cert,
		componentCAName:   ca,
	}

	if err := CreateOrUpdateSecretWithOwnerRef(secretName, cr.Namespace, componentSecretData, cr.K8sClient, cr.OwnerRef); err != nil {
		log.Error(err, "Unable to create secret for component")
		return
	}
}

func (cr *CertificateRequest) GenerateKibanaCerts(componentName string) {
	secret, err := getSecret(kibanaSecretName, cr.Namespace, cr.K8sClient)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "unable to get secret")
		return
	}

	kibanaCert := &certificate{}
	if err := unmarshalCert(secret.Data[kibanaComponentCertName], secret.Data[kibanaComponentKeyName], kibanaCert); err != nil {
		log.Info("Failed to unmarshal cert from secret for kibana", "error", err)
	}

	err = cr.EnsureCert(kibanaComponentName, kibanaCert)
	if err != nil {
		log.Error(err, "Unable to generate cert for kibana")
		return
	}

	ca, err := cr.getCACertBytes()
	if err != nil {
		log.Error(err, "Unable to get CA bytes")
		return
	}

	kibanaSecretData := map[string][]byte{
		kibanaComponentKeyName:  kibanaCert.key,
		kibanaComponentCertName: kibanaCert.cert,
		kibanaComponentCAName:   ca,
	}

	if err := CreateOrUpdateSecretWithOwnerRef(kibanaSecretName, cr.Namespace, kibanaSecretData, cr.K8sClient, cr.OwnerRef); err != nil {
		log.Error(err, "Unable to create secret for kibana component")
		return
	}

	secret, err = getSecret(getKibanaProxySecretName(kibanaSecretName), cr.Namespace, cr.K8sClient)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "unable to get secret")
		return
	}

	kibanaProxyCert := &certificate{}
	if err := unmarshalCert(secret.Data[kibanaInternalCertName], secret.Data[kibanaInternalKeyName], kibanaProxyCert); err != nil {
		log.Info("Failed to unmarshal cert from secret for kibana-proxy", "error", err)
	}

	err = cr.EnsureCert(kibanaInternalComponentName, kibanaProxyCert)
	if err != nil {
		log.Error(err, "Unable to generate cert for kibana-internal")
		return
	}

	kibanaProxySessionSecret := &sessionSecret{}
	kibanaProxySessionSecret.secret = secret.Data[kibanaInternalSessionSecretName]

	// ensure session secret here
	err = ensureSessionSecret(kibanaSessionSecretLength, allowedRunes, kibanaProxySessionSecret)
	if err != nil {
		log.Error(err, "Unable to ensure session secret for kibana proxy")
		return
	}

	secretData := map[string][]byte{
		kibanaInternalSessionSecretName: kibanaProxySessionSecret.secret,
		kibanaInternalCertName:          kibanaProxyCert.cert,
		kibanaInternalKeyName:           kibanaProxyCert.key,
	}

	if err := CreateOrUpdateSecretWithOwnerRef(getKibanaProxySecretName(kibanaSecretName), cr.Namespace, secretData, cr.K8sClient, cr.OwnerRef); err != nil {
		log.Error(err, "Unable to create secret for kibana-proxy")
		return
	}
}

func (cr *CertificateRequest) GenerateElasticsearchCerts(clusterName string) {
	// get from secret
	secret, err := getSecret(clusterName, cr.Namespace, cr.K8sClient)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "unable to get secret")
		return
	}

	adminCert := &certificate{}

	if err := unmarshalCert(secret.Data[esAdminCertName], secret.Data[esAdminKeyName], adminCert); err != nil {
		log.Info("Failed to unmarshal cert from secret for admin", "error", err)
	}

	err = cr.EnsureCert(esAdminComponentName, adminCert)
	if err != nil {
		log.Error(err, "Unable to generate cert for admin user")
		return
	}

	elasticsearchCert := &certificate{}
	if err := unmarshalCert(secret.Data[esComponentCertName], secret.Data[esComponentKeyName], elasticsearchCert); err != nil {
		log.Info("Failed to unmarshal cert from secret for elasticsearch", "error", err)
	}

	err = cr.EnsureCert(esComponentName, elasticsearchCert)
	if err != nil {
		log.Error(err, "Unable to generate cert for elasticsearch")
		return
	}

	loggingESCert := &certificate{}
	if err := unmarshalCert(secret.Data[esInternalCertname], secret.Data[esInternalKeyName], loggingESCert); err != nil {
		log.Info("Failed to unmarshal cert from secret for logging-es", "error", err)
	}

	err = cr.EnsureCert(esInternalComponentName, loggingESCert)
	if err != nil {
		log.Error(err, "Unable to generate cert for logging-es")
		return
	}

	ca, err := cr.getCACertBytes()
	if err != nil {
		log.Error(err, "Unable to get CA bytes")
		return
	}

	secretData := map[string][]byte{
		esComponentKeyName:  elasticsearchCert.key,
		esComponentCertName: elasticsearchCert.cert,
		esInternalKeyName:   loggingESCert.key,
		esInternalCertname:  loggingESCert.cert,
		esAdminKeyName:      adminCert.key,
		esAdminCertName:     adminCert.cert,
		esAdminCAName:       ca,
	}

	if err := CreateOrUpdateSecretWithOwnerRef(clusterName, cr.Namespace, secretData, cr.K8sClient, cr.OwnerRef); err != nil {
		log.Error(err, "Unable to create secret for elasticsearch component")
		return
	}
}

func (cr *CertificateRequest) persistCA(caCert *certCA) error {
	secretName := cr.getSigningSecretName()

	secretData := map[string][]byte{
		esCACertName:   caCert.cert,
		esCAKeyName:    caCert.key,
		esCASerialName: []byte(caCert.serial.Text(10)),
	}

	return CreateOrUpdateSecretWithOwnerRef(secretName, cr.Namespace, secretData, cr.K8sClient, cr.OwnerRef)
}

func (cr *CertificateRequest) ensureCA(caCert *certCA) error {
	caCert.certMutex.Lock()
	defer caCert.certMutex.Unlock()

	secretName := cr.getSigningSecretName()

	// get the ca from the secret if we can
	secret, err := getSecret(secretName, cr.Namespace, cr.K8sClient)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		ca, err := validateCASecret(secret)
		if err != nil {
			return err
		}

		caCert.x509Cert = ca.x509Cert
		caCert.privKey = ca.privKey
		caCert.cert = ca.cert
		caCert.key = ca.key
		caCert.pubKeySHA1 = ca.pubKeySHA1
		caCert.serial = ca.serial
	}

	// check if the CA cert is invalid
	if !isValidCA(caCert.x509Cert, caCert.privKey) {
		// generate new CLO CA and populate the CA secret with it
		ca, err := genCA()
		if err != nil {
			return err
		}

		caCert.x509Cert = ca.x509Cert
		caCert.privKey = ca.privKey
		caCert.cert = ca.cert
		caCert.key = ca.key
		caCert.pubKeySHA1 = ca.pubKeySHA1
		caCert.serial = ca.serial

		return cr.persistCA(caCert)
	}

	return nil
}

func (cr *CertificateRequest) incrementCertSerial(ca *certCA) (*big.Int, error) {
	ca.certMutex.Lock()
	defer ca.certMutex.Unlock()

	ca.serial.Add(ca.serial, bigOne)
	serial := big.NewInt(0)
	serial.Set(ca.serial)

	if err := cr.persistCA(ca); err != nil {
		return nil, err
	}

	return serial, nil
}

func (cr *CertificateRequest) getCACertBytes() ([]byte, error) {
	ca := &certCA{}
	err := cr.ensureCA(ca)
	if err != nil {
		return []byte{}, err
	}

	return ca.cert, nil
}

func (cr *CertificateRequest) EnsureCert(componentName string, cert *certificate) error {
	// validate that the cert isn't expired
	if !isValidCert(cert.x509Cert, cert.privKey, componentName) {
		err := cr.generateCert(componentName, cert)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cr *CertificateRequest) generateCert(componentName string, cert *certificate) error {
	ca := &certCA{}
	err := cr.ensureCA(ca)
	if err != nil {
		return err
	}

	privKey, err := rsa.GenerateKey(rand.Reader, rsaKeyLength)
	if err != nil {
		return err
	}

	pubKeySHA1 := sha1.Sum(x509.MarshalPKCS1PublicKey(&privKey.PublicKey))

	serial, err := cr.incrementCertSerial(ca)
	if err != nil {
		return err
	}

	cert.certMutex.Lock()
	defer cert.certMutex.Unlock()

	x509Cert := &x509.Certificate{
		SerialNumber:       serial,
		SignatureAlgorithm: x509.SHA512WithRSA,
		Subject: pkix.Name{
			Organization:       certOrganization,
			OrganizationalUnit: componentOrganizationUnit,
			CommonName:         componentName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(compNotAfterYears, 0, -1),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
		SubjectKeyId:          pubKeySHA1[:],
		AuthorityKeyId:        ca.pubKeySHA1[:],
	}

	ext := cr.Extensions[componentName]
	if ext.dns != nil {
		x509Cert.DNSNames = ext.dns
	}
	if ext.ips != nil {
		x509Cert.IPAddresses = ext.ips
	}

	if ext.oid {
		// build our own SAN entry instead and append it to ret ExtraExtensions

		// the following is abstracted from x509.go : buildCertExtensions & marshalSANs
		var rawValues []asn1.RawValue
		for _, name := range ext.dns {
			rawValues = append(rawValues, asn1.RawValue{Tag: nameTypeDNS, Class: 2, Bytes: []byte(name)})
		}
		for _, rawIP := range ext.ips {
			// If possible, we always want to encode IPv4 addresses in 4 bytes.
			ip := rawIP.To4()
			if ip == nil {
				ip = rawIP
			}
			rawValues = append(rawValues, asn1.RawValue{Tag: nameTypeIP, Class: 2, Bytes: ip})
		}
		// add the rID
		rawValues = append(rawValues, ridRawValue)

		sanExtensionValue, err := asn1.Marshal(rawValues)
		if err != nil {
			return err
		}

		sanExtension := pkix.Extension{
			Id:       sanIdentifier,
			Critical: sanCriticality,
			Value:    sanExtensionValue,
		}

		x509Cert.ExtraExtensions = append(x509Cert.ExtraExtensions, sanExtension)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, x509Cert, ca.x509Cert, &privKey.PublicKey, ca.privKey)
	if err != nil {
		return err
	}

	pemCert, err := pemEncodeCert(certBytes)
	if err != nil {
		return err
	}

	pemKey, err := pemEncodePrivateKey(privKey)
	if err != nil {
		return err
	}

	cert.x509Cert = x509Cert
	cert.privKey = privKey
	cert.cert = pemCert
	cert.key = pemKey

	return nil
}

func certWillExpireSoon(cert *x509.Certificate) bool {
	certExpiration := cert.NotAfter
	return time.Now().After(certExpiration.Add(time.Hour * -1))
}

func genCA() (*certCA, error) {
	caPrivKey, err := rsa.GenerateKey(rand.Reader, rsaKeyLength)
	if err != nil {
		return nil, err
	}
	caPubKeySHA1 := sha1.Sum(x509.MarshalPKCS1PublicKey(&caPrivKey.PublicKey))
	serial := big.NewInt(0)
	ca := &x509.Certificate{
		SerialNumber:       serial,
		SignatureAlgorithm: x509.SHA512WithRSA,
		Subject: pkix.Name{
			Country:            caCountry,
			Organization:       certOrganization,
			OrganizationalUnit: caOrganizationUnit,
			CommonName:         caCN,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(caNotAfterYears, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		SubjectKeyId:          caPubKeySHA1[:],
		AuthorityKeyId:        caPubKeySHA1[:],
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}
	caPEMBytes, err := pemEncodeCert(caBytes)
	if err != nil {
		return nil, err
	}
	keyPEMBytes, err := pemEncodePrivateKey(caPrivKey)
	if err != nil {
		return nil, err
	}
	return &certCA{
		certificate{
			caPEMBytes,
			keyPEMBytes,
			ca,
			caPrivKey,
			sync.Mutex{},
		},
		serial,
		caPubKeySHA1[:],
	}, nil
}

func isValidCert(x509Cert *x509.Certificate, rsaPrivKey *rsa.PrivateKey, commonName string) bool {
	if x509Cert == nil {
		return false
	}

	if rsaPrivKey == nil {
		return false
	}

	if certWillExpireSoon(x509Cert) {
		return false
	}

	rsaPubKey, ok := x509Cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return false
	}

	if rsaPubKey.N.Cmp(rsaPrivKey.N) != 0 {
		return false
	}

	if x509Cert.Issuer.CommonName != caCN {
		return false
	}

	if x509Cert.Subject.CommonName != commonName {
		return false
	}

	return true
}

func isValidCA(x509Cert *x509.Certificate, rsaPrivKey *rsa.PrivateKey) bool {
	if !isValidCert(x509Cert, rsaPrivKey, caCN) {
		return false
	}

	if !x509Cert.IsCA {
		return false
	}

	return true
}

func pemEncodePrivateKey(privKey *rsa.PrivateKey) ([]byte, error) {
	pemBuffer := &bytes.Buffer{}
	if err := pem.Encode(pemBuffer, &pem.Block{
		Type:  `RSA PRIVATE KEY`,
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}); err != nil {
		return nil, err
	}
	return pemBuffer.Bytes(), nil
}

func pemDecodePrivateKey(keyBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	if block.Type == "RSA PRIVATE KEY" {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}

	if block.Type == "PRIVATE KEY" {
		pkcs8Key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}

		privateKey, ok := pkcs8Key.(*rsa.PrivateKey)
		if ok {
			return privateKey, nil
		}
	}

	return nil, fmt.Errorf("failed to decode PEM block containing private key")
}

func pemEncodeCert(certDERBytes []byte) ([]byte, error) {
	pemBuffer := &bytes.Buffer{}
	if err := pem.Encode(pemBuffer, &pem.Block{Type: `CERTIFICATE`, Bytes: certDERBytes}); err != nil {
		return nil, err
	}
	return pemBuffer.Bytes(), nil
}

func pemDecodeCert(certPEMBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEMBytes)
	if block == nil || block.Type != `CERTIFICATE` {
		return nil, fmt.Errorf("failed to decode PEM block containing certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

func getKibanaProxySecretName(componentName string) string {
	return fmt.Sprintf("%s-proxy", componentName)
}

func ensureSessionSecret(secretLength int, runes []rune, session *sessionSecret) error {
	if secretLength <= 0 {
		return nil
	}

	if session != nil {
		expression, err := regexp.Compile(fmt.Sprintf("^[%s]{%d}$", string(runes), secretLength))
		if err != nil {
			return err
		}

		if expression.MatchString(string(session.secret)) {
			return nil
		}
	}

	// otherwise its different, regenerate it
	b := make([]rune, secretLength)
	for i := range b {
		b[i] = runes[mathRand.Intn(len(runes))]
	}

	session.secret = []byte(string(b))
	return nil
}

func unmarshalCert(cert, key []byte, unmarshalledCert *certificate) error {
	// if we are providing empty cert or key, just return
	if len(cert) == 0 || len(key) == 0 {
		return nil
	}

	x509Cert, err := pemDecodeCert(cert)
	if err != nil {
		return err
	}

	rsaKey, err := pemDecodePrivateKey(key)
	if err != nil {
		return err
	}

	unmarshalledCert.cert = cert
	unmarshalledCert.key = key
	unmarshalledCert.x509Cert = x509Cert
	unmarshalledCert.privKey = rsaKey

	return nil
}

func validateCASecret(secret *v1.Secret) (*certCA, error) {
	var x509Cert *x509.Certificate
	var err error
	certBytes, certOK := secret.Data[esCACertName]
	if !certOK {
		return nil, fmt.Errorf("missing cert key from secret")
	}
	decodedCert, err := pemDecodeCert(certBytes)
	if err != nil {
		return nil, err
	}
	x509Cert = decodedCert

	var rsaKey *rsa.PrivateKey
	keyBytes, keyOK := secret.Data[esCAKeyName]
	if !keyOK {
		return nil, fmt.Errorf("missing key key from secret")
	}
	decodedKey, err := pemDecodePrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}
	rsaKey = decodedKey

	pubKeySHA1 := sha1.Sum(x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey))
	serial := big.NewInt(0)
	serialBytes, serialOK := secret.Data[esCASerialName]

	if !serialOK {
		return nil, fmt.Errorf("missing serial key from secret")
	}
	if err = serial.UnmarshalText(serialBytes); err != nil {
		return nil, err
	}

	if !isValidCA(x509Cert, rsaKey) {
		return nil, fmt.Errorf("invalid CA")
	}
	return &certCA{
		certificate{
			certBytes,
			keyBytes,
			x509Cert,
			rsaKey,
			sync.Mutex{},
		},
		serial,
		pubKeySHA1[:],
	}, nil
}
