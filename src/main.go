package main

import (
	"fmt"
	"path/filepath"
)

func main() {
	jsonPath := filepath.Join(KEYS_JSON)
	ch := GetKeyDataAsync(jsonPath, "vk_user1_openai")
	data := <-ch
	fmt.Println(data.ApiKey, data.Provider)
}
