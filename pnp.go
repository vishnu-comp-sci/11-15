package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

func main() {
	// Specify the service to search for
	service := "ssdp:all"

	// Create a UDP address for SSDP multicast
	ssdp, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		panic(err)
	}

	// Open a UDP connection
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Set a deadline for the read operation
	conn.SetReadDeadline(time.Now().Add(time.Second * 3))

	// Send an M-SEARCH request
	msg := []byte(
		"M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			"ST:" + service + "\r\n" +
			"MAN: \"ssdp:discover\"\r\n" +
			"MX: 1\r\n" +
			"\r\n",
	)

	_, err = conn.WriteToUDP(msg, ssdp)
	if err != nil {
		panic(err)
	}

	// Buffer to store responses
	var responses []string

	// Listen for responses until the deadline is reached
	for {
		buf := make([]byte, 2048)
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				break // Deadline reached, stop listening
			}
			panic(err)
		}

		responses = append(responses, string(buf[:n]))
	}

	// Process responses to find and print the location URL of InternetGatewayDevice

	locationURL := printInternetGatewayDeviceLocation(responses)
	newURL := locationURL + "/ctl/IPConn"
	fmt.Println(newURL)
	addPortMapping(newURL)
}

// Function to parse SSDP response and print the location of InternetGatewayDevice
func printInternetGatewayDeviceLocation(ssdpResponses []string) string {
	var locationURL string
	for _, response := range ssdpResponses {
		if strings.Contains(response, "ST: urn:schemas-upnp-org:device:InternetGatewayDevice") {
			scanner := bufio.NewScanner(bytes.NewBufferString(response))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "LOCATION:") {
					fmt.Println(strings.TrimSpace(strings.TrimPrefix(line, "LOCATION:")))
					locationURL = strings.TrimSpace(strings.TrimPrefix(line, "LOCATION:"))
					break
				}
			}
		}

	}
	return locationURL
}

func addPortMapping(newURL string) {
	var newExtPort string
	fmt.Print("Enter new external port: ")
	fmt.Scanln(&newExtPort)

	var newIntPort string
	fmt.Print("Enter new internal port: ")
	fmt.Scanln(&newIntPort)

	var newProtocol string
	fmt.Print("Enter new protocol (TCP or UDP): ")
	fmt.Scanln(&newProtocol)

	// Build request body
	type AddPortMapping struct {
		NewRemoteHost             string `xml:"NewRemoteHost"`
		NewExternalPort           string `xml:"NewExternalPort"`
		NewProtocol               string `xml:"NewProtocol"`
		NewInternalPort           string `xml:"NewInternalPort"`
		NewInternalClient         string `xml:"NewInternalClient"`
		NewEnabled                string `xml:"NewEnabled"`
		NewPortMappingDescription string `xml:"NewPortMappingDescription"`
		NewLeaseDuration          string `xml:"NewLeaseDuration"`
	}
	body := AddPortMapping{
		NewRemoteHost:             "",
		NewExternalPort:           newExtPort,
		NewProtocol:               newProtocol,
		NewInternalPort:           newIntPort,
		NewInternalClient:         "192.168.1.2", // Replace with your internal client IP
		NewEnabled:                "1",
		NewPortMappingDescription: "TestMapping",
		NewLeaseDuration:          "10", // 0 for indefinite
	}
	type SOAPBody struct {
		XMLName xml.Name       `xml:"s:Body"`
		Content AddPortMapping `xml:"u:AddPortMapping"`
	}

	// Define the SOAP envelope
	type SOAPEnvelope struct {
		XMLName xml.Name `xml:"s:Envelope"`
		S       string   `xml:"xmlns:s,attr"`
		U       string   `xml:"xmlns:u,attr"`
		Body    SOAPBody
	}

	envelope := SOAPEnvelope{
		XMLName: xml.Name{Local: "s:Envelope"},
		S:       "http://schemas.xmlsoap.org/soap/envelope/",
		U:       "urn:schemas-upnp-org:service:WANIPConnection:1",
		Body:    SOAPBody{Content: body},
	}

	buf := new(bytes.Buffer)
	enc := xml.NewEncoder(buf)
	enc.Indent("", "  ")
	if err := enc.Encode(envelope); err != nil {
		fmt.Println("error:", err)
		return
	}

	reqBody := buf.String()

	// Send request
	req, err := http.NewRequest("POST", newURL, bytes.NewBufferString(reqBody))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	req.Header.Set("SOAPAction", "urn:schemas-upnp-org:service:WANIPConnection:1#AddPortMapping")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	// Read and print the response body
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Response status:", resp.Status)
	fmt.Println("Response body:", string(bodyBytes))
}
