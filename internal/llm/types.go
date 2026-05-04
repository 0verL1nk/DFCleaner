package llm

type LLMConfig struct {
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	APIKey    string `json:"apiKey"`
	ModelName string `json:"modelName"`
	IsActive  bool   `json:"isActive"`
}

type ConnectionTestResult struct {
	Success           bool   `json:"success"`
	Error             string `json:"error,omitempty"`
	NoFunctionCalling bool   `json:"noFunctionCalling"`
	ModelInfo         string `json:"modelInfo,omitempty"`
}
