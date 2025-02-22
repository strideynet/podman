package integration

import (
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/containers/podman/v3/test/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman run with --ip flag", func() {
	var (
		tempdir    string
		err        error
		podmanTest *PodmanTestIntegration
	)

	BeforeEach(func() {
		SkipIfRootless("rootless does not support --ip without network")
		tempdir, err = CreateTempDirInTempDir()
		if err != nil {
			os.Exit(1)
		}
		podmanTest = PodmanTestCreate(tempdir)
		podmanTest.Setup()
		podmanTest.SeedImages()
		// Cleanup the CNI networks used by the tests
		os.RemoveAll("/var/lib/cni/networks/podman")
	})

	AfterEach(func() {
		podmanTest.Cleanup()
		f := CurrentGinkgoTestDescription()
		processTestResult(f)

	})

	It("Podman run --ip with garbage address", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "114232346", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("Podman run --ip with v6 address", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "2001:db8:bad:beef::1", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("Podman run --ip with non-allocatable IP", func() {
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", "203.0.113.124", ALPINE, "ls"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})

	It("Podman run with specified static IP has correct IP", func() {
		ip := GetRandomIPAddress()
		result := podmanTest.Podman([]string{"run", "-ti", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
	})

	It("Podman run with --network bridge:ip=", func() {
		ip := GetRandomIPAddress()
		result := podmanTest.Podman([]string{"run", "-ti", "--network", "bridge:ip=" + ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
	})

	It("Podman run with --network net:ip=,mac=,interface_name=", func() {
		ip := GetRandomIPAddress()
		mac := "44:33:22:11:00:99"
		intName := "myeth"
		result := podmanTest.Podman([]string{"run", "-ti", "--network", "bridge:ip=" + ip + ",mac=" + mac + ",interface_name=" + intName, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		Expect(result.OutputToString()).To(ContainSubstring(ip + "/16"))
		Expect(result.OutputToString()).To(ContainSubstring(mac))
		Expect(result.OutputToString()).To(ContainSubstring(intName))
	})

	It("Podman run two containers with the same IP", func() {
		ip := GetRandomIPAddress()
		result := podmanTest.Podman([]string{"run", "-dt", "--ip", ip, nginx})
		result.WaitWithDefaultTimeout()
		Expect(result).Should(Exit(0))
		for i := 0; i < 10; i++ {
			fmt.Println("Waiting for nginx", err)
			time.Sleep(1 * time.Second)
			response, err := http.Get(fmt.Sprintf("http://%s", ip))
			if err != nil {
				continue
			}
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		result = podmanTest.Podman([]string{"run", "-ti", "--ip", ip, ALPINE, "ip", "addr"})
		result.WaitWithDefaultTimeout()
		Expect(result).To(ExitWithError())
	})
})
