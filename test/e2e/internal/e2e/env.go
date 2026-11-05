//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultPortBase = 30000
	mysqlRootPass   = "PoleE2E123!"
)

type Options struct {
	Suite       string
	PortBase    int
	ConsoleOpen bool
	ClientOpen  bool
}

type Env struct {
	T           *testing.T
	Root        string
	TempDir     string
	MySQLPort   int
	APIPort     int
	ConsolePort int

	ConsoleBaseURL string
	ClientBaseURL  string

	mysqlContainer testcontainers.Container
	cmd            *exec.Cmd
	stdoutPath     string
	stderrPath     string
}

func Start(t *testing.T, opts Options) *Env {
	t.Helper()
	ctx := context.Background()
	if opts.PortBase == 0 {
		opts.PortBase = defaultPortBase
	}
	root := RepoRoot(t)
	tempDir := t.TempDir()
	ports := NewPortAllocator(opts.PortBase)
	env := &Env{
		T:           t,
		Root:        root,
		TempDir:     tempDir,
		MySQLPort:   ports.Next(t),
		APIPort:     ports.Next(t),
		ConsolePort: ports.Next(t),
	}
	env.ConsoleBaseURL = fmt.Sprintf("http://127.0.0.1:%d", env.ConsolePort)
	env.ClientBaseURL = fmt.Sprintf("http://127.0.0.1:%d", env.APIPort)

	env.startMySQL(ctx)
	env.writeConfig(opts)
	env.startControlPlane()
	t.Cleanup(func() {
		env.stop(ctx)
	})
	return env
}

func RepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate e2e helper file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	return root
}

func (e *Env) startMySQL(ctx context.Context) {
	e.T.Helper()
	mysqlPort := "3306/tcp"
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8.0.36",
		ExposedPorts: []string{mysqlPort},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": mysqlRootPass,
			"MYSQL_DATABASE":      "pole_server",
			"MYSQL_ROOT_HOST":     "%",
		},
		WaitingFor: wait.ForListeningPort(mysqlPort).WithStartupTimeout(2 * time.Minute),
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.PortBindings = network.PortMap{
				network.MustParsePort(mysqlPort): []network.PortBinding{{
					HostIP:   netip.MustParseAddr("127.0.0.1"),
					HostPort: strconv.Itoa(e.MySQLPort),
				}},
			}
		},
	}
	mysql, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		e.T.Fatalf("start mysql container on %d: %v", e.MySQLPort, err)
	}
	e.mysqlContainer = mysql

	dsn := fmt.Sprintf("root:%s@tcp(127.0.0.1:%d)/?multiStatements=true&parseTime=true&loc=Local", mysqlRootPass, e.MySQLPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		e.T.Fatalf("open mysql: %v", err)
	}
	defer db.Close()
	Eventually(e.T, 2*time.Minute, 500*time.Millisecond, func() bool {
		return db.Ping() == nil
	}, "mysql accepts connections")

	schema, err := os.ReadFile(filepath.Join(e.Root, "plugin/store/mysql/scripts/pole_server.sql"))
	if err != nil {
		e.T.Fatalf("read mysql schema: %v", err)
	}
	if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS pole_observability DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;"); err != nil {
		e.T.Fatalf("create observability db: %v", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		e.T.Fatalf("apply mysql schema: %v", err)
	}
}

func (e *Env) writeConfig(opts Options) {
	e.T.Helper()
	apiConfigPath := filepath.Join(e.TempDir, "pole-apiserver.yaml")
	serverConfigPath := filepath.Join(e.TempDir, "pole-server.yaml")
	logPath := filepath.Join(e.Root, "deploy/conf/pole-log.yaml")
	dbAddr := fmt.Sprintf("127.0.0.1:%d", e.MySQLPort)
	dsn := fmt.Sprintf("root:%s@tcp(%s)/pole_server?loc=Local&parseTime=true", mysqlRootPass, dbAddr)

	apiConfig := fmt.Sprintf(`- name: api-http
  option:
    listenIP: "127.0.0.1"
    listenPort: %d
    enablePprof: false
    enableSwagger: false
    connLimit:
      openConnLimit: false
      maxConnPerHost: 128
      maxConnLimit: 5120
      whiteList: 127.0.0.1
      purgeCounterInterval: 10s
      purgeCounterExpired: 5s
    enableCacheProto: false
    sizeCacheProto: 128
  api:
    admin:
      enable: true
    aimcp:
      enable: true
      include: [default]
    aia2a:
      enable: true
      include: [default]
    console:
      enable: true
      include: [default, service, config]
    client:
      enable: true
      include: [discover, register, healthcheck, config]
`, e.APIPort)
	if err := os.WriteFile(apiConfigPath, []byte(apiConfig), 0o644); err != nil {
		e.T.Fatalf("write api config: %v", err)
	}

	serverConfig := fmt.Sprintf(`bootstrap:
  logger: %s
  mode: all
  console:
    logger:
      ErrorOutputPaths:
        - %s
      RotateOutputPath: %s
      RotationMaxSize: 50
      RotationMaxAge: 3
      RotationMaxBackups: 3
      level: info
    webServer:
      mode: release
      listenIP: 127.0.0.1
      listenPort: %d
      jwt:
        secretKey: pole-e2e-secret
        expired: 7200
      coreURL: /core/v1
      namingURL: /naming/v1
      authURL: /auth/v1
      configURL: /config/v1
      monitorURL: /api/v1
      mainUser: admin
    store:
      name: mysql
      option:
        master:
          dbType: mysql
          dbName: pole_observability
          dbUser: root
          dbPwd: %s
          dbAddr: %s
          maxOpenConns: 30
          maxIdleConns: 10
          connMaxLifetime: 300
          txIsolationLevel: 2
    poleServer:
      address: 127.0.0.1:%d
    monitorServer:
      address: 127.0.0.1:%d
  startInOrder:
    open: false
  polaris_service:
    enable_register: false
apiservers: %s
auth:
  user:
    name: defaultUser
    option:
      salt: polarismesh@2021
  strategy:
    name: defaultStrategy
    option:
      consoleOpen: %t
      consoleStrict: true
      clientOpen: %t
      clientStrict: false
namespace:
  autoCreate: true
naming:
  autoCreate: true
  batch:
    register:
      open: false
    deregister:
      open: false
  healthcheck:
    open: true
    service: pole.checker
    slotNum: 30
    minCheckInterval: 1s
    maxCheckInterval: 30s
    clientReportInterval: 120s
    batch:
      heartbeat:
        open: false
    checkers:
      - name: heartbeat
config:
  open: true
cache:
  diffTime: 1s
maintain:
  jobs:
    - name: DeleteUnHealthyInstance
      enable: false
    - name: DeleteEmptyAutoCreatedService
      enable: false
    - name: CleanDeletedResources
      enable: false
store:
  name: defaultStore
  option:
    master:
      dbType: mysql
      dns: "%s"
      maxOpenConns: 30
      maxIdleConns: 10
      connMaxLifetime: 300
plugin:
  crypto:
    entries:
      - name: AES
  cmdb:
    name: memory
    option:
      url: ""
      interval: 60s
  history:
    entries:
      - name: HistoryLogger
  discoverEvent:
    entries:
      - name: EventLogger
  statis:
    entries:
      - name: local
        option:
          interval: 60
  ratelimit:
    name: token-bucket
    option:
      enable: false
`, logPath,
		filepath.Join(e.TempDir, "pole-console-error.log"),
		filepath.Join(e.TempDir, "pole-console.log"),
		e.ConsolePort,
		mysqlRootPass,
		dbAddr,
		e.APIPort,
		e.APIPort+1000,
		apiConfigPath,
		opts.ConsoleOpen,
		opts.ClientOpen,
		dsn,
	)
	if err := os.WriteFile(serverConfigPath, []byte(serverConfig), 0o644); err != nil {
		e.T.Fatalf("write server config: %v", err)
	}
}

func (e *Env) startControlPlane() {
	e.T.Helper()
	cfg := filepath.Join(e.TempDir, "pole-server.yaml")
	e.stdoutPath = filepath.Join(e.TempDir, "control-plane.stdout.log")
	e.stderrPath = filepath.Join(e.TempDir, "control-plane.stderr.log")
	stdout, err := os.Create(e.stdoutPath)
	if err != nil {
		e.T.Fatalf("create stdout log: %v", err)
	}
	stderr, err := os.Create(e.stderrPath)
	if err != nil {
		e.T.Fatalf("create stderr log: %v", err)
	}
	cmd := exec.Command("go", "run", ".", "start", "-c", cfg, "--mode", "all")
	cmd.Dir = e.Root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("MYSQL_USER=%s", "root"),
		fmt.Sprintf("MYSQL_PWD=%s", mysqlRootPass),
		fmt.Sprintf("MYSQL_HOST=127.0.0.1:%d", e.MySQLPort),
	)
	if err := cmd.Start(); err != nil {
		e.T.Fatalf("start control-plane: %v", err)
	}
	e.cmd = cmd
	e.T.Cleanup(func() {
		_ = stdout.Close()
		_ = stderr.Close()
	})
	Eventually(e.T, 2*time.Minute, 500*time.Millisecond, func() bool {
		resp, err := http.Get(e.ConsoleBaseURL + "/admin/v1/console/ability")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, "console ability endpoint is ready")
	Eventually(e.T, 2*time.Minute, 500*time.Millisecond, func() bool {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", e.APIPort), 250*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, "api-http port is ready")
}

func (e *Env) stop(ctx context.Context) {
	if e.cmd != nil && e.cmd.Process != nil {
		_ = e.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- e.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = e.cmd.Process.Kill()
			<-done
		}
	}
	if e.mysqlContainer != nil {
		_ = e.mysqlContainer.Terminate(ctx)
	}
}

func (e *Env) LogTail() string {
	var out bytes.Buffer
	for _, p := range []string{e.stdoutPath, e.stderrPath} {
		b, _ := os.ReadFile(p)
		if len(b) > 4096 {
			b = b[len(b)-4096:]
		}
		out.WriteString("\n--- " + filepath.Base(p) + " ---\n")
		out.Write(b)
	}
	return out.String()
}
