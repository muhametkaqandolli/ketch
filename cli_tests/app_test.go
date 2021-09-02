// +build integration

package cli_tests

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	appCliName       = "sample-app"
	appYamlName      = "sample-yaml-app"
	appCliFramework  = "appframework"
	appYamlFramework = "appframework-yaml"
	appImage         = "gcr.io/shipa-ci/sample-go-app:latest"
	cName            = "my-cname.com"
	testEnvvarKey    = "FOO"
	testEnvVarValue  = "BAR"
)

func TestAppHelp(t *testing.T) {
	b, err := exec.Command(ketch, "app", "--help").CombinedOutput()
	require.Nil(t, err, string(b))
	require.Contains(t, string(b), "deploy")
	require.Contains(t, string(b), "export")
	require.Contains(t, string(b), "info")
	require.Contains(t, string(b), "list")
	require.Contains(t, string(b), "log")
	require.Contains(t, string(b), "remove")
	require.Contains(t, string(b), "start")
	require.Contains(t, string(b), "stop")
}

func TestAppByCli(t *testing.T) {
	defer func() {
		cleanupApp(appCliName)
		time.Sleep(time.Second) // give app time to be removed
		cleanupFramework(appCliFramework)
	}()

	// app framework
	b, err := exec.Command(ketch, "framework", "add", appCliFramework, "--ingress-service-endpoint", ingress, "--ingress-type", "traefik").CombinedOutput()
	require.Nil(t, err, string(b))
	err = retry(ketch, []string{"framework", "list"}, "", fmt.Sprintf("%s[ \t]+Created", appCliFramework), 3, 3)
	require.Nil(t, err, string(b))

	// app deploy
	b, err = exec.Command(ketch, "app", "deploy", appCliName, "--framework", appCliFramework, "-i", appImage).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Equal(t, "", string(b))

	// app unit set
	b, err = exec.Command(ketch, "app", "deploy", appCliName, "--framework", appCliFramework, "-i", appImage, "--units", "3").CombinedOutput()
	require.Nil(t, err, string(b))

	b, err = exec.Command("kubectl", "describe", "apps", appCliName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.True(t, regexp.MustCompile("Units:  3").Match(b), string(b)) // note two spaces

	// app info
	err = retry(ketch, []string{"app", "info", appCliName}, "", fmt.Sprintf("1[ \t]+%s[ \t]+web[ \t]+100%%[ \t]+3 running", appImage), 20, 5)
	require.Nil(t, err)

	// app list
	err = retry(ketch, []string{"app", "list"}, "", fmt.Sprintf("%s[ \t]+%s[ \t]+3 running", appCliName, appCliFramework), 4, 4)
	require.Nil(t, err)

	b, err = exec.Command(ketch, "app", "list").CombinedOutput()
	require.Nil(t, err, string(b))

	require.True(t, regexp.MustCompile("NAME[ \t]+FRAMEWORK[ \t]+STATE[ \t]+ADDRESSES[ \t]+BUILDER[ \t]+DESCRIPTION").Match(b), string(b))
	require.True(t, regexp.MustCompile(fmt.Sprintf("%s[ \t]+%s[ \t]+(.*[1-3] running)", appCliName, appCliFramework)).Match(b), string(b))

	// app export
	b, err = exec.Command(ketch, "app", "export", appCliName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Contains(t, string(b), fmt.Sprintf("framework: %s", appCliFramework))
	require.Contains(t, string(b), fmt.Sprintf("image: %s", appImage))
	require.Contains(t, string(b), fmt.Sprintf("name: %s", appCliName))
	require.Contains(t, string(b), "type: Application")

	b, err = exec.Command(ketch, "app", "info", appCliName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.True(t, regexp.MustCompile("DEPLOYMENT VERSION[ \t]+IMAGE[ \t]+PROCESS NAME[ \t]+WEIGHT[ \t]+STATE[ \t]+CMD").Match(b), string(b))
	require.True(t, regexp.MustCompile(fmt.Sprintf("1[ \t]+%s[ \t]+web[ \t]+100%%[ \t]+.*[1-3] running[ \t]", appImage)).Match(b), string(b))

	// app stop
	b, err = exec.Command(ketch, "app", "stop", appCliName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Equal(t, "Successfully stopped!\n", string(b))

	// app start
	b, err = exec.Command(ketch, "app", "start", appCliName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Equal(t, "Successfully started!\n", string(b))

	// app log
	err = exec.Command(ketch, "app", "log", appCliName).Run()
	require.Nil(t, err)

	// app cname
	err = exec.Command(ketch, "cname", "add", cName, "--app", appCliName).Run()
	require.Nil(t, err)
	b, err = exec.Command(ketch, "app", "info", appCliName).CombinedOutput()
	require.Nil(t, err)
	require.True(t, regexp.MustCompile(fmt.Sprintf("Address: http://%s", cName)).Match(b), string(b))

	// app env set
	err = exec.Command(ketch, "env", "set", fmt.Sprintf("%s=%s", testEnvvarKey, testEnvVarValue), "--app", appCliName).Run()
	require.Nil(t, err)

	// app env get
	b, err = exec.Command(ketch, "env", "get", testEnvvarKey, "--app", appCliName).CombinedOutput()
	require.Nil(t, err)
	require.Contains(t, string(b), testEnvVarValue, string(b))

	// app env unset
	err = exec.Command(ketch, "env", "unset", testEnvvarKey, "--app", appCliName).Run()
	require.Nil(t, err)
	b, err = exec.Command(ketch, "env", "get", testEnvvarKey, "--app", appCliName).CombinedOutput()
	require.Nil(t, err)
	require.NotContainsf(t, string(b), testEnvVarValue, string(b))

	// app remove
	b, err = exec.Command(ketch, "app", "remove", appCliName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Contains(t, string(b), "Successfully removed!")
	err = retry(ketch, []string{"app", "info", appCliName}, "", "not found", 4, 4)
	require.Nil(t, err)

	// framework remove
	err = retry(ketch, []string{"framework", "remove", appCliFramework}, fmt.Sprintf("ketch-%s", appCliFramework), "Framework successfully removed!", 3, 8)
	require.Nil(t, err)
}

func TestAppByYaml(t *testing.T) {
	defer func() {
		cleanupApp(appYamlName)
		time.Sleep(time.Second) // give app time to be removed
		cleanupFramework(appYamlFramework)
	}()

	// app framework
	b, err := exec.Command(ketch, "framework", "add", appYamlFramework, "--ingress-service-endpoint", ingress, "--ingress-type", "traefik").CombinedOutput()
	require.Nil(t, err, string(b))
	err = retry(ketch, []string{"framework", "list"}, "", fmt.Sprintf("%s[ \t]+Created", appYamlFramework), 3, 3)
	require.Nil(t, err, string(b))

	// app deploy
	temp, err := os.CreateTemp(t.TempDir(), "*.yaml")
	require.Nil(t, err)
	defer os.Remove(temp.Name())
	_, err = temp.WriteString(fmt.Sprintf(`name: "%s"
version: v1
type: Application
image: %s
framework: %s
description: cli test app by yaml`, appYamlName, appImage, appYamlFramework))
	require.Nil(t, err)
	b, err = exec.Command(ketch, "app", "deploy", temp.Name()).CombinedOutput()
	require.Nil(t, err, string(b))

	err = retry(ketch, []string{"app", "info", appYamlName}, "", "running", 20, 7)
	require.Nil(t, err)

	// // app unit set
	// b, err = exec.Command(ketch, "app", "deploy", appYamlName, "--framework", appYamlFramework, "-i", appImage, "--units", "3").CombinedOutput()
	// require.Nil(t, err, string(b))

	b, err = exec.Command("kubectl", "describe", "apps", appYamlName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.True(t, regexp.MustCompile("Units:  1").Match(b), string(b)) // note two spaces

	// app info
	err = retry(ketch, []string{"app", "info", appYamlName}, "", fmt.Sprintf("1[ \t]+%s[ \t]+web[ \t]+100%%[ \t]+1 running", appImage), 20, 5)
	require.Nil(t, err)

	// app list
	err = retry(ketch, []string{"app", "list"}, "", fmt.Sprintf("%s[ \t]+%s[ \t]+1 running", appYamlName, appYamlFramework), 4, 4)
	require.Nil(t, err)

	b, err = exec.Command(ketch, "app", "list").CombinedOutput()
	require.Nil(t, err, string(b))

	require.True(t, regexp.MustCompile("NAME[ \t]+FRAMEWORK[ \t]+STATE[ \t]+ADDRESSES[ \t]+BUILDER[ \t]+DESCRIPTION").Match(b), string(b))
	require.True(t, regexp.MustCompile(fmt.Sprintf("%s[ \t]+%s[ \t]+1 running", appYamlName, appYamlFramework)).Match(b), string(b))

	// app export
	b, err = exec.Command(ketch, "app", "export", appYamlName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Contains(t, string(b), fmt.Sprintf("framework: %s", appCliFramework))
	require.Contains(t, string(b), fmt.Sprintf("image: %s", appImage))
	require.Contains(t, string(b), fmt.Sprintf("name: %s", appYamlName))
	require.Contains(t, string(b), "type: Application")
	require.Contains(t, string(b), "version: v1")

	b, err = exec.Command(ketch, "app", "info", appYamlName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.True(t, regexp.MustCompile("DEPLOYMENT VERSION[ \t]+IMAGE[ \t]+PROCESS NAME[ \t]+WEIGHT[ \t]+STATE[ \t]+CMD").Match(b), string(b))
	require.True(t, regexp.MustCompile(fmt.Sprintf("1[ \t]+%s[ \t]+web[ \t]+100%%[ \t]+1 running[ \t]", appImage)).Match(b), string(b))

	// app stop
	b, err = exec.Command(ketch, "app", "stop", appYamlName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Equal(t, "Successfully stopped!\n", string(b))

	// app start
	b, err = exec.Command(ketch, "app", "start", appYamlName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Equal(t, "Successfully started!\n", string(b))

	// app log
	err = exec.Command(ketch, "app", "log", appYamlName).Run()
	require.Nil(t, err)

	// app cname
	err = exec.Command(ketch, "cname", "add", cName, "--app", appYamlName).Run()
	require.Nil(t, err)
	b, err = exec.Command(ketch, "app", "info", appYamlName).CombinedOutput()
	require.Nil(t, err)
	require.True(t, regexp.MustCompile(fmt.Sprintf("Address: http://%s", cName)).Match(b), string(b))

	// app env set
	err = exec.Command(ketch, "env", "set", fmt.Sprintf("%s=%s", testEnvvarKey, testEnvVarValue), "--app", appYamlName).Run()
	require.Nil(t, err)

	// app env get
	b, err = exec.Command(ketch, "env", "get", testEnvvarKey, "--app", appYamlName).CombinedOutput()
	require.Nil(t, err)
	require.Contains(t, string(b), testEnvVarValue, string(b))

	// app env unset
	err = exec.Command(ketch, "env", "unset", testEnvvarKey, "--app", appYamlName).Run()
	require.Nil(t, err)
	b, err = exec.Command(ketch, "env", "get", testEnvvarKey, "--app", appYamlName).CombinedOutput()
	require.Nil(t, err)
	require.NotContainsf(t, string(b), testEnvVarValue, string(b))

	// app remove
	b, err = exec.Command(ketch, "app", "remove", appYamlName).CombinedOutput()
	require.Nil(t, err, string(b))
	require.Contains(t, string(b), "Successfully removed!")
	err = retry(ketch, []string{"app", "info", appYamlName}, "", "not found", 4, 4)
	require.Nil(t, err)

	// framework remove
	// retry - will error that apps are still present sometimes
	err = retry(ketch, []string{"framework", "remove", appYamlFramework}, fmt.Sprintf("ketch-%s", appYamlFramework), "Framework successfully removed!", 3, 8)
	require.Nil(t, err)
}