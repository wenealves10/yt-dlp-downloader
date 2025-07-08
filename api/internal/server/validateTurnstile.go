package server

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type TurnstileRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
	RemoteIP string `json:"remoteip,omitempty"`
}

type TurnstileResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func (s *Server) validateTurnstile(token string, ip string) bool {
	reqBody := TurnstileRequest{
		Secret:   s.config.TurnstileSecret,
		Response: token,
		RemoteIP: ip,
	}

	jsonBody, _ := json.Marshal(reqBody)
	resp, err := http.Post("https://challenges.cloudflare.com/turnstile/v0/siteverify", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result TurnstileResponse
	json.NewDecoder(resp.Body).Decode(&result)

	return result.Success
}
