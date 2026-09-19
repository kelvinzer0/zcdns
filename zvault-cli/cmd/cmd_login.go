package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zvault-cli/internal/api"
	"zvault-cli/internal/config"
)

func cmdLogin() {
	resp, err := http.Post(api.BaseURL+"/vault/auth/device", "application/json", nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	var d api.DeviceAuthResp
	json.NewDecoder(resp.Body).Decode(&d)

	fmt.Printf("Please open: %s\n", d.VerificationUriComplete)
	fmt.Printf("Or open %s and enter code: %s\n", d.VerificationUri, d.UserCode)

	for {
		time.Sleep(2 * time.Second)
		reqBody := fmt.Sprintf(`{"device_code":"%s"}`, d.DeviceCode)
		pResp, err := http.Post(api.BaseURL+"/vault/auth/poll", "application/json", strings.NewReader(reqBody))
		if err != nil {
			continue
		}
		var p api.PollResp
		json.NewDecoder(pResp.Body).Decode(&p)
		pResp.Body.Close()

		if p.Status == "approved" {
			cfg := &config.Config{Token: p.Token, Subdomain: p.Subdomain}
			config.SaveConfig(cfg)
			fmt.Printf("Successfully logged in as %s\n", p.Subdomain)
			return
		} else if p.Status == "denied" || p.Status == "expired" {
			fmt.Println("Auth failed:", p.Status)
			return
		}
	}
}
