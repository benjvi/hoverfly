package hoverfly_end_to_end_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os/exec"
	"os"
	"strings"
	"github.com/phayes/freeport"
	"fmt"
	"github.com/dghubble/sling"
	"strconv"
	"io/ioutil"
)

var _ = Describe("When I use hoverfly-cli", func() {
	var (
		hoverflyCmd *exec.Cmd

		workingDir, _ = os.Getwd()
		adminPort = freeport.GetPort()
		adminPortAsString = strconv.Itoa(adminPort)

		proxyPort = freeport.GetPort()
	)

	Describe("with a running hoverfly", func() {

		BeforeEach(func() {
			hoverflyCmd = startHoverfly(adminPort, proxyPort, workingDir)
		})

		AfterEach(func() {
			hoverflyCmd.Process.Kill()
		})

		Describe("Managing Hoverflies data using the CLI", func() {

			BeforeEach(func() {
				DoRequest(sling.New().Post(fmt.Sprintf("http://localhost:%v/api/records", adminPort)).Body(strings.NewReader(`
					{
						"data": [{
							"request": {
								"path": "/api/bookings",
								"method": "POST",
								"destination": "www.my-test.com",
								"scheme": "http",
								"query": "",
								"body": "{\"flightId\": \"1\"}",
								"headers": {
									"Content-Type": [
										"application/json"
									]
								}
							},
							"response": {
								"status": 201,
								"body": "",
								"encodedBody": false,
								"headers": {
									"Location": [
										"http://localhost/api/bookings/1"
									]
								}
							}
						}]
					}`)))

				resp := DoRequest(sling.New().Get(fmt.Sprintf("http://localhost:%v/api/records", adminPort)))
				bytes, _ := ioutil.ReadAll(resp.Body)
				Expect(string(bytes)).ToNot(Equal(`{"data":null}`))
			})

			It("can export, delete and then re-import the data", func() {

				// Export the data
				output, err := exec.Command(hoverctlBinary, "export", "mogronalol/twitter", "--admin-port=" + adminPortAsString).Output()
				Expect(err).To(BeNil())
				Expect(output).To(ContainSubstring("mogronalol/twitter:latest exported successfully"))
				Expect(ioutil.ReadFile(hoverctlCacheDir + "/mogronalol.twitter.latest.json")).To(MatchJSON(`
					{
						"data": [{
							"request": {
								"path": "/api/bookings",
								"method": "POST",
								"destination": "www.my-test.com",
								"scheme": "http",
								"query": "",
								"body": "{\"flightId\": \"1\"}",
								"headers": {
									"Content-Type": [
										"application/json"
									]
								}
							},
							"response": {
								"status": 201,
								"body": "",
								"encodedBody": false,
								"headers": {
									"Location": [
										"http://localhost/api/bookings/1"
									]
								}
							}
						}]
					}`),
				)

				// Wipe it
				output, err = exec.Command(hoverctlBinary, "delete", "simulations", "--admin-port=" + adminPortAsString).Output()
				Expect(err).To(BeNil())
				Expect(output).To(ContainSubstring("Simulations have been deleted from Hoverfly"))

				resp := DoRequest(sling.New().Get(fmt.Sprintf("http://localhost:%v/api/records", adminPort)))
				bytes, _ := ioutil.ReadAll(resp.Body)
				Expect(string(bytes)).To(Equal(`{"data":null}`))

				// Re-import it
				output, err = exec.Command(hoverctlBinary, "import", "mogronalol/twitter", "--admin-port=" + adminPortAsString).Output()
				Expect(err).To(BeNil())
				Expect(output).To(ContainSubstring("mogronalol/twitter:latest imported successfully"))

				resp = DoRequest(sling.New().Get(fmt.Sprintf("http://localhost:%v/api/records", adminPort)))
				bytes, _ = ioutil.ReadAll(resp.Body)
				Expect(string(bytes)).To(MatchJSON(`
					{
						"data": [{
							"request": {
								"path": "/api/bookings",
								"method": "POST",
								"destination": "www.my-test.com",
								"scheme": "http",
								"query": "",
								"body": "{\"flightId\": \"1\"}",
								"headers": {
									"Content-Type": [
										"application/json"
									]
								}
							},
							"response": {
								"status": 201,
								"body": "",
								"encodedBody": false,
								"headers": {
									"Location": [
										"http://localhost/api/bookings/1"
									]
								}
							}
						}]
					}`))
			})

		})
	})
})