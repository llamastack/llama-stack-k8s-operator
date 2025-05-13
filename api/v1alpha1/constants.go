/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"os"
	"strings"
)

// GetDistributionImage returns the image for a given distribution name
func GetDistributionImage(distributionName string) string {
	// Convert distribution name to environment variable format
	// e.g., "vllm-gpu" -> "VLLM_GPU_IMAGE"
	envVar := strings.ToUpper(strings.ReplaceAll(distributionName, "-", "_")) + "_IMAGE"
	return os.Getenv(envVar)
}

// GetAvailableDistributions returns a map of all available distributions and their images
func GetAvailableDistributions() map[string]string {
	distributions := make(map[string]string)

	// Read all environment variables
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}

		key, value := pair[0], pair[1]

		// Check if this is a distribution image environment variable
		if strings.HasSuffix(key, "_IMAGE") {
			// Convert environment variable name to distribution name
			// e.g., "VLLM_GPU_IMAGE" -> "vllm-gpu"
			distName := strings.ToLower(strings.TrimSuffix(key, "_IMAGE"))
			distName = strings.ReplaceAll(distName, "_", "-")
			distributions[distName] = value
		}
	}

	return distributions
}

// ValidateDistribution checks if a distribution name is valid
func ValidateDistribution(name string) bool {
	_, exists := GetAvailableDistributions()[name]
	return exists
}
