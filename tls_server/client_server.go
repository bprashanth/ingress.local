package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	rsaBits  = 2048
	validFor = 365 * 24 * time.Hour
)

// generateRSACerts generates a basic self signed certificate using a key length
// of rsaBits, valid for validFor time.
func generateRSACerts(host string, isCA bool, keyOut, certOut io.Writer) error {
	if len(host) == 0 {
		return fmt.Errorf("Require a non-empty host for client hello")
	}
	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return fmt.Errorf("Failed to generate key: %v", err)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	if err != nil {
		return fmt.Errorf("failed to generate serial number: %s", err)
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("Failed to create certificate: %s", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("Failed creating cert: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return fmt.Errorf("Failed creating keay: %v", err)
	}
	return nil
}

// simpleGET executes a get on the given url, returns error if non-200 returned.
func simpleGET(c *http.Client, url, host string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Host = host
	res, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	rawBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	body := string(rawBody)
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf(
			"GET returned http error %v", res.StatusCode)
	}
	return body, err
}

// buildHTTPSClient creates a client capable of performing HTTPS requests.
// Note that the given rootCA must be configured with isCA=true.
func buildHTTPSClient(serverName string, rootCA []byte) (*http.Client, error) {
	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(rootCA)
	if !ok {
		return nil, fmt.Errorf("Unable to load serverCA.")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         serverName,
			RootCAs:            pool,
		},
	}
	return &http.Client{Transport: tr}, nil
}

// listenAndServeTLS essentially implements http.ListenAndServeTLS, but plugs
// in the byte values of the key and cert instead of reading them from file.
// Another notable difference between this function and the one in stdlib is
// that this one does *not* establish keep-alive sessions.
func listenAndServeTLS(key, cert []byte) (err error) {
	addr := ":443"
	srv := &http.Server{Addr: addr}
	config := &tls.Config{}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.X509KeyPair(cert, key)
	if err != nil {
		return err
	}

	var ln net.Listener
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return srv.Serve(tls.NewListener(ln, config))
}

// startServer starts a simple hello world server that performs tls handshakes
// with the given key/cert pair.
func startServer(key, cert []byte) {
	http.HandleFunc("/hello",
		func(w http.ResponseWriter, req *http.Request) {
			io.WriteString(w, "hello, world!\n")
		})
	go func() {
		log.Printf("Starting https server on :443")
		err := listenAndServeTLS(key, cert)
		if err != nil {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()
}

func main() {
	var k, c bytes.Buffer
	if err := generateRSACerts("foo.bar.com", true, &k, &c); err != nil {
		log.Fatal(err)
	}

	key := k.Bytes()
	cert := c.Bytes()
	startServer(key, cert)

	client, err := buildHTTPSClient("foo.bar.com", cert)
	if err != nil {
		log.Fatal(err)
	}

	for sleepTime := 1 * time.Second; sleepTime < 1*time.Minute; sleepTime = 2 * sleepTime {
		resp, err := simpleGET(client, "https://localhost/hello", "foo.bar.com")
		if err == nil {
			log.Print(resp)
			return
		}
		log.Printf("Sleeping for %v, Error: %v", sleepTime, err)
		time.Sleep(sleepTime)
	}
}
