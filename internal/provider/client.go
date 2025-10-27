package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func CallFlowsAPI[ReqT any, ResT any](providerConfigData FlowsProviderConfiguredData, urlPath string, req ReqT) (*ResT, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequest("POST", providerConfigData.Endpoint+urlPath, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+providerConfigData.Token)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	respData, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data  ResT   `json:"data,omitempty"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, fmt.Errorf("could not json-decode response: %w; response: %s", err, string(respData))
	}
	if response.Error != "" {
		return nil, errors.New(response.Error)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", httpResponse.StatusCode)
	}

	return &response.Data, nil
}
