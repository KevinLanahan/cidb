package runner

import (
	"fmt"
	"net/http"
	"os"
)

func DeleteSession(slug string) error {
	supabaseURL := os.Getenv("SUPABASE_URL")
	anonKey := os.Getenv("SUPABASE_ANON_KEY")
	if supabaseURL == "" || anonKey == "" {
		return fmt.Errorf("SUPABASE_URL and SUPABASE_ANON_KEY must be set in .env")
	}

	url := supabaseURL + "/rest/v1/sessions?slug=eq." + slug

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("delete failed (status %d) — check your Supabase keys", resp.StatusCode)
	}

	return nil
}

func LoadEnvForDelete() {
	loadEnv()
}
