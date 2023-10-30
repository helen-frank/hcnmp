package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os/exec"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/helen-frank/hcnmp/pkg/server/servererror"
	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
	"github.com/helen-frank/hcnmp/pkg/zone/proxy"
)

func (h *handler) podNetConnectServer(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	containerName := c.Query("container")
	server := c.Query("server")
	if server == "" {
		servererror.HandleError(c, http.StatusBadRequest, errors.New("server cannot be empty"))
		return
	}
	port := c.Query("port")

	client, err := proxy.GetClusterPorxyClientFromCode(c.Param("clusterCode"))
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	execCmd := []string{"sh", "-c", "curl --connect-timeout 1 " + server}
	if port != "" {
		execCmd[len(execCmd)-1] += ":" + port
	}

	if _, stderr, err := execute(client, namespace, name, containerName, execCmd); err != nil {
		if _, ok := err.(apierrors.APIStatus); ok {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}

		curlErr := err.Error()
		klog.Errorf("failed to exec command, err: %v, stderr: %v", err, string(stderr))
		curlNotFound := false
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 127 {
			curlNotFound = true
		}

		// Try using nc and ping
		if port != "" {
			execCmd = []string{"sh", "-c", "nc -z -w 1 " + server + " " + port}
		} else {
			execCmd = []string{"sh", "-c", "ping -W 1 -c 1 " + server}
		}

		if _, stderr, err := execute(client, namespace, name, containerName, execCmd); err != nil {
			if _, ok := err.(apierrors.APIStatus); ok {
				servererror.HandleError(c, http.StatusInternalServerError, err)
				return
			}

			execCmdStr := "nc"
			if len(port) == 0 {
				execCmdStr = "ping"
			}

			cmdNotFound := false
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 127 {
				cmdNotFound = true
			}

			if curlNotFound {
				if cmdNotFound {
					err = errors.New("curl and " + execCmdStr + " not found")
				} else {
					err = errors.New("curl not found and exec " + execCmdStr + " err: " + err.Error() + ", stderr: " + string(stderr))
				}
			} else {
				strTemp := "exec curl failed, err: " + curlErr + " and "
				if cmdNotFound {
					err = errors.New(strTemp + execCmdStr + " not found")
				} else {
					err = errors.New(strTemp + "exec " + execCmdStr + " err: " + err.Error() + ", stderr: " + string(stderr))
				}
			}
			servererror.HandleError(c, http.StatusBadRequest, err)
			return
		}
	}

	c.JSON(http.StatusOK, nil)
}

func execute(client *clientset.Clientset, namespace, name, containerName string, execCmd []string) (stdout, stderr []byte, err error) {
	req := client.CoreV1().RESTClient().Post().
		Name(name).
		Resource("pods").
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   execCmd,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(client.ClientConfig(), http.MethodPost, req.URL())
	if err != nil {
		return nil, nil, err
	}

	stdoutBuf := bytes.NewBuffer([]byte{})
	stderrBuf := bytes.NewBuffer([]byte{})
	if err := exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: stdoutBuf,
		Stderr: stderrBuf,
		Tty:    false,
	}); err != nil {
		return nil, nil, err
	}

	if stdout, err = io.ReadAll(stdoutBuf); err != nil {
		return nil, nil, err
	}

	if stderr, err = io.ReadAll(stderrBuf); err != nil {
		return nil, nil, err
	}

	return
}
