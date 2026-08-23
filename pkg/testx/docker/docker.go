// Package docker 提供测试期间启动与停止 docker 容器的能力。
package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Container 记录测试用容器的名字与宿主机访问地址
type Container struct {
	Name     string
	HostPort string
}

// StartContainer 启动指定容器供测试使用。
// 每个测试可能运行在独立进程中,进程间无法串行化此调用,
// 故先查固定名容器是否已存在;启动失败时退避等待其他进程把容器拉起来。
func StartContainer(image string, name string, port string, dockerArgs []string, appArgs []string) (Container, error) {
	// 容器已在运行则直接复用
	if c, err := exists(name, port); err == nil {
		return c, nil
	}

	c, err := dockerRun(image, name, port, dockerArgs, appArgs)
	if err == nil {
		return c, nil
	}

	// 启动失败多半是同名校验冲突:另一个测试进程已占名,
	// 退避轮询等待其容器就绪
	for i := range 10 {
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)

		c, err := exists(name, port)
		if err == nil {
			return c, nil
		}
	}

	return Container{}, fmt.Errorf("could not start or find container %s", name)
}

// StopContainer 停止并删除指定容器(含挂载卷)
func StopContainer(id string) error {
	// #nosec G204 -- 测试基建,容器 id 由 docker daemon 返回,非外部输入
	if err := exec.Command("docker", "stop", id).Run(); err != nil {
		return fmt.Errorf("could not stop container: %w", err)
	}

	// #nosec G204 -- 同上
	if err := exec.Command("docker", "rm", id, "-v").Run(); err != nil {
		return fmt.Errorf("could not remove container: %w", err)
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
	arg = append(arg, "run", "-P", "-d", "--name", name)
	arg = append(arg, dockerArgs...)
	arg = append(arg, image)
	arg = append(arg, appArgs...)

	var out bytes.Buffer
	// #nosec G204 -- 测试基建,镜像与参数由测试代码传入,非外部输入
	cmd := exec.Command("docker", arg...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Container{}, fmt.Errorf("could not start container %s: %w", image, err)
	}

	id := out.String()[:12]
	hostIP, hostPort, err := extractIPPort(id, port)
	if err != nil {
		_ = StopContainer(id)
		return Container{}, fmt.Errorf("could not extract ip/port: %w", err)
	}

	c := Container{
		Name:     name,
		HostPort: net.JoinHostPort(hostIP, hostPort),
	}

	return c, nil
}

// exists 通过 inspect 判断固定名容器是否在运行,在运行则返回其访问地址
func exists(name string, port string) (Container, error) {
	hostIP, hostPort, err := extractIPPort(name, port)
	if err != nil {
		return Container{}, errors.New("container not running")
	}

	c := Container{
		Name:     name,
		HostPort: net.JoinHostPort(hostIP, hostPort),
	}

	return c, nil
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
