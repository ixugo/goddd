// Package docker 提供测试期间启动与停止 docker 容器的能力。
package docker

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

// Container 记录测试用容器的 ID、名字与宿主机访问地址
type Container struct {
	ID       string
	Name     string
	HostPort string
}

// StartContainer 启动唯一ID容器供测试使用，测后清理。
// fix: 使用唯一容器名称（<名称前缀>-<当前进程 PID>-<随机后缀>），避免产生脏容器。容器占用等潜在错误退出。
// 每个测试可能运行在独立进程中,进程间无法串行化此调用,
// 故先查固定名容器是否已存在;启动失败时退避等待其他进程把容器拉起来。
func StartContainer(image string, name string, port string, dockerArgs []string, appArgs []string) (Container, error) {
	containerName, err := randomContainerName(name)
	if err != nil {
		return Container{}, fmt.Errorf("生成测试容器名称失败: %w", err)
	}

	c, err := dockerRun(image, containerName, port, dockerArgs, appArgs)
	if err == nil {
		return c, nil
	}

	return Container{}, fmt.Errorf("启动测试容器失败 %s: %w", name, err)
}

// StopContainer 强制停止并删除指定测试容器(含挂载卷),重复调用视为成功。
func StopContainer(id string) error {
	// #nosec G204 -- 同上
	cmd := exec.Command("docker", "rm", "-f", "-v", id)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No such container") {
		return fmt.Errorf("删除测试容器失败: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// DumpContainerLogs 输出运行中容器的日志,用于测试失败时排查
func DumpContainerLogs(id string) []byte {
	// #nosec G204 -- 同上
	out, err := exec.Command("docker", "logs", id).CombinedOutput()
	if err != nil {
		return nil
	}

	return out
}

// dockerRun 以后台守护方式启动容器,-P 由 daemon 随机映射宿主机端口,
// 避免固定端口在多环境下冲突
func dockerRun(image string, name string, port string, dockerArgs []string, appArgs []string) (Container, error) {
	arg := make([]string, 0, 6+len(dockerArgs)+len(appArgs))
	arg = append(arg, "run", "--rm", "-P", "-d", "--name", name)
	arg = append(arg, dockerArgs...)
	arg = append(arg, image)
	arg = append(arg, appArgs...)

	var out bytes.Buffer
	// #nosec G204 -- 测试基建,镜像与参数由测试代码传入,非外部输入
	cmd := exec.Command("docker", arg...)
	output, err := cmd.CombinedOutput()
	out.Write(output)
	if err != nil {
		return Container{}, fmt.Errorf("could not start container %s: %w: %s", image, err, strings.TrimSpace(string(output)))
	}

	id := strings.TrimSpace(out.String())
	if id == "" {
		return Container{}, errors.New("docker run 未返回容器 ID")
	}
	hostIP, hostPort, err := extractIPPort(id, port)
	if err != nil {
		_ = StopContainer(id)
		return Container{}, fmt.Errorf("could not extract ip/port: %w", err)
	}

	c := Container{
		ID:       id,
		Name:     name,
		HostPort: net.JoinHostPort(hostIP, hostPort),
	}

	return c, nil
}

// randomContainerName 生成 Docker 合法的唯一名称
func randomContainerName(prefix string) (string, error) {
	prefix = strings.ToLower(strings.Trim(prefix, "-"))
	if prefix == "" {
		prefix = "testx"
	}
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	if len(prefix) > 36 {
		prefix = prefix[:36]
	}
	return fmt.Sprintf("%s-%d-%s", prefix, os.Getpid(), hex.EncodeToString(suffix[:])), nil
}

// extractIPPort 用 docker inspect 的 Go template 取出容器端口在宿主机的
// 随机映射地址;兼容 IPv6 双绑定与 Podman 空 HostIP 的差异
func extractIPPort(name string, port string) (hostIP string, hostPort string, err error) {
	tmpl := fmt.Sprintf("[{{range $k,$v := (index .NetworkSettings.Ports \"%s/tcp\")}}{{json $v}}{{end}}]", port)

	var out bytes.Buffer
	// #nosec G204 -- 测试基建,容器名由测试代码传入,非外部输入
	cmd := exec.Command("docker", "inspect", "-f", tmpl, name)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("could not inspect container %s: %w", name, err)
	}

	// 开启 IPv6 时 inspect 输出为连续两段对象:
	// Got  [{"HostIp":"0.0.0.0","HostPort":"49190"}{"HostIp":"::","HostPort":"49190"}]
	// Need [{"HostIp":"0.0.0.0","HostPort":"49190"},{"HostIp":"::","HostPort":"49190"}]
	data := strings.ReplaceAll(out.String(), "}{", "},{")

	var docs []struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}
	if err := json.Unmarshal([]byte(data), &docs); err != nil {
		return "", "", fmt.Errorf("could not decode json: %w", err)
	}

	for _, doc := range docs {
		if doc.HostIP != "::" {
			// Podman 的 HostIP 为空而非 0.0.0.0
			// - https://github.com/containers/podman/issues/17780
			if doc.HostIP == "" {
				return "localhost", doc.HostPort, nil
			}

			return doc.HostIP, doc.HostPort, nil
		}
	}

	return "", "", fmt.Errorf("could not locate ip/port")
}
