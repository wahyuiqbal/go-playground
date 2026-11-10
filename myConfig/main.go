package main

import (
	"fmt"
	"os"
	"strconv"

	beegoConfig "github.com/beego/beego/v2/core/config"
	"github.com/beego/beego/v2/server/web"
	vault "github.com/hashicorp/vault/api"
)

type appConfig struct {
	AppName                string `json:"app_name"`
	RunMode                string `json:"run_mode"`
	HTTPPort               int    `json:"http_port"`
	Graceful               bool   `json:"graceful"`
	CopyRequestBody        bool   `json:"copy_request_body"`
	EnableAdmin            bool   `json:"enable_admin"`
	LogPath                string `json:"log_path"`
	PlatformMaskingDataKey string `json:"platformmaskingdatakey"`
}

var config appConfig

type MainController struct {
	web.Controller
}

func (c *MainController) Get() {
	key, err := beegoConfig.String("platformmaskingdatakey")
	if err != nil || key == "" {
		key = config.PlatformMaskingDataKey
	}

	c.Ctx.ResponseWriter.WriteHeader(200)
	_, _ = c.Ctx.ResponseWriter.Write([]byte(fmt.Sprintf("Hello from Beego with JSON config! platformmaskingdata::key=%s", key)))
}

func loadVaultConfig() (appConfig, error) {
	cfg := appConfig{
		AppName:         "myConfig",
		RunMode:         "dev",
		HTTPPort:        8080,
		Graceful:        true,
		CopyRequestBody: true,
		EnableAdmin:     false,
		LogPath:         "/var/log/myapp.log",
	}

	fmt.Println("Loading configuration from Vault...")

	vaultAddr := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")
	vaultPath := os.Getenv("VAULT_SECRET_PATH")
	fmt.Printf("Loading configuration from Vault at %s, path: %s\n", vaultAddr, vaultPath)
	if vaultAddr == "" || vaultToken == "" || vaultPath == "" {
		return cfg, nil
	}

	client, err := vault.NewClient(&vault.Config{Address: vaultAddr})
	if err != nil {
		return cfg, fmt.Errorf("create vault client: %w", err)
	}
	client.SetToken(vaultToken)

	secret, err := client.Logical().Read(vaultPath)
	if err != nil {
		return cfg, fmt.Errorf("read vault secret %s: %w", vaultPath, err)
	}
	if secret == nil || secret.Data == nil {
		return cfg, fmt.Errorf("vault secret %s returned no data", vaultPath)
	}

	data := map[string]interface{}{}
	if rawData, ok := secret.Data["data"]; ok {
		if mapped, ok := rawData.(map[string]interface{}); ok {
			data = mapped
		}
	}
	if len(data) == 0 {
		for k, v := range secret.Data {
			data[k] = v
		}
	}

	if v, ok := data["app_name"].(string); ok && v != "" {
		cfg.AppName = v
	}
	if v, ok := data["run_mode"].(string); ok && v != "" {
		cfg.RunMode = v
	}
	if v, ok := data["http_port"]; ok {
		if i, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil {
			cfg.HTTPPort = i
		}
	}
	if v, ok := data["graceful"]; ok {
		if b, err := strconv.ParseBool(fmt.Sprintf("%v", v)); err == nil {
			cfg.Graceful = b
		}
	}
	if v, ok := data["copy_request_body"]; ok {
		if b, err := strconv.ParseBool(fmt.Sprintf("%v", v)); err == nil {
			cfg.CopyRequestBody = b
		}
	}
	if v, ok := data["log_path"].(string); ok && v != "" {
		cfg.LogPath = v
	}

	return cfg, nil
}

func initConfig() error {
	loadedConfig, err := loadVaultConfig()
	if err != nil {
		return fmt.Errorf("load vault config: %w", err)
	}
	config = loadedConfig

	web.BConfig.AppName = config.AppName
	web.BConfig.RunMode = config.RunMode
	web.BConfig.Listen.HTTPPort = config.HTTPPort
	web.BConfig.Listen.Graceful = config.Graceful
	web.BConfig.CopyRequestBody = config.CopyRequestBody
	web.BConfig.Listen.EnableAdmin = config.EnableAdmin
	web.BConfig.Listen.EnableHTTP = true

	return nil
}

func main() {
	if err := initConfig(); err != nil {
		panic(err)
	}

	fmt.Printf("Starting %s in %s mode on port %d\n", config.AppName, config.RunMode, config.HTTPPort)
	fmt.Printf("Graceful shutdown: %v, CopyRequestBody: %v, EnableAdmin: %v\n", config.Graceful, config.CopyRequestBody, config.EnableAdmin)
	fmt.Printf("Log path: %s\n", config.LogPath)
	web.Router("/", &MainController{})
	web.Run()
}
