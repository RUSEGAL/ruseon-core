package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/RUSEGAL/ruseon-core/pkg/config"
)

var (
	apiURL   string
	apiUser  string
	apiPass  string
	apiToken string
)

// login fetches a JWT token if username/password are provided
func login() error {
	if apiUser == "" || apiPass == "" {
		return nil // No auth provided, try without (or might fail with 401)
	}

	creds := map[string]string{
		"username": apiUser,
		"password": apiPass,
	}
	b, _ := json.Marshal(creds)

	resp, err := http.Post(fmt.Sprintf("%s/login", apiURL), "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status: %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if token, ok := result["token"]; ok {
		apiToken = token
		return nil
	}
	return fmt.Errorf("no token in login response")
}

// doRequest helper to add auth headers and execute request
func doRequest(method, endpoint string, body []byte) (*http.Response, error) {
	if apiToken == "" && apiUser != "" {
		if err := login(); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s/%s", apiURL, endpoint), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}

	return http.DefaultClient.Do(req)
}

var rootCmd = &cobra.Command{
	Use:   "ruseon-cli",
	Short: "Command line interface for RUSEON Core",
	Long:  `ruseon-cli is a CLI tool for managing cameras and viewing metrics on a running RUSEON Core server.`,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get server status and metrics",
	RunE: func(_ *cobra.Command, _ []string) error {
		resp, err := doRequest(http.MethodGet, "stats", nil)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %d", resp.StatusCode)
		}

		var stats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			return err
		}

		fmt.Println("RUSEON Core Status:")
		fmt.Printf("  Uptime:         %.0f seconds\n", stats["uptime"])
		fmt.Printf("  Memory Used:    %.2f MB\n", stats["memoryUsed"].(float64)/1024/1024)
		fmt.Printf("  Total Cameras:  %.0f\n", stats["totalCameras"])
		fmt.Printf("  Online Cameras: %.0f\n", stats["onlineCameras"])
		fmt.Printf("  Active Clients: %.0f\n", stats["activeClients"])
		return nil
	},
}

var camerasCmd = &cobra.Command{
	Use:   "cameras",
	Short: "Manage cameras",
}

var listCamerasCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cameras",
	RunE: func(_ *cobra.Command, _ []string) error {
		resp, err := doRequest(http.MethodGet, "cameras", nil)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %d", resp.StatusCode)
		}

		var cameras []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&cameras); err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSTATUS\tRECORD\tUPTIME\tURL")
		
		for _, cam := range cameras {
			status := "OFFLINE"
			if state, ok := cam["state"].(string); ok && state == "online" {
				status = "ONLINE"
			}
			if disabled, ok := cam["disabled"].(bool); ok && disabled {
				status = "DISABLED"
			}
			
			record := "NO"
			if rec, ok := cam["record"].(bool); ok && rec {
				record = "YES"
			}
			
			uptime := float64(0)
			if up, ok := cam["uptime"].(float64); ok {
				uptime = up
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%.0fs\t%s\n", cam["id"], status, record, uptime, cam["url"])
		}
		return w.Flush()
	},
}

var (
	camID     string
	camURL    string
	camRecord bool
)

var addCameraCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new camera",
	RunE: func(_ *cobra.Command, _ []string) error {
		if camID == "" || camURL == "" {
			return fmt.Errorf("--id and --url are required")
		}

		reqBody := config.CameraConfig{
			ID:     camID,
			URL:    camURL,
			Record: camRecord,
		}

		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}

		resp, err := doRequest(http.MethodPost, "cameras", b)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
		}

		fmt.Printf("Camera '%s' added successfully.\n", camID)
		return nil
	},
}

var deleteCameraCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a camera",
	RunE: func(_ *cobra.Command, _ []string) error {
		if camID == "" {
			return fmt.Errorf("--id is required")
		}

		resp, err := doRequest(http.MethodDelete, fmt.Sprintf("cameras/%s", camID), nil)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
		}

		fmt.Printf("Camera '%s' deleted successfully.\n", camID)
		return nil
	},
}

func init() {
	// ENV defaults
	defaultUser := os.Getenv("RUSEON_USER")
	defaultPass := os.Getenv("RUSEON_PASS")

	rootCmd.PersistentFlags().StringVar(&apiURL, "api", "http://127.0.0.1:8080/api", "API URL of the RUSEON server")
	rootCmd.PersistentFlags().StringVarP(&apiUser, "user", "u", defaultUser, "Username for authentication")
	rootCmd.PersistentFlags().StringVarP(&apiPass, "pass", "p", defaultPass, "Password for authentication")
	
	rootCmd.AddCommand(statusCmd)
	
	camerasCmd.AddCommand(listCamerasCmd)
	
	addCameraCmd.Flags().StringVar(&camID, "id", "", "Camera ID")
	addCameraCmd.Flags().StringVar(&camURL, "url", "", "RTSP URL")
	addCameraCmd.Flags().BoolVar(&camRecord, "record", false, "Enable recording")
	camerasCmd.AddCommand(addCameraCmd)
	
	deleteCameraCmd.Flags().StringVar(&camID, "id", "", "Camera ID")
	camerasCmd.AddCommand(deleteCameraCmd)

	rootCmd.AddCommand(camerasCmd)

	// Users commands
	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
	}

	listUsersCmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := doRequest(http.MethodGet, "users", nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server returned status: %d: %s", resp.StatusCode, string(body))
			}

			var users []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "USERNAME\tROLE")
			for _, u := range users {
				fmt.Fprintf(w, "%s\t%s\n", u["username"], u["role"])
			}
			return w.Flush()
		},
	}

	var (
		usrUsername string
		usrPassword string
		usrRole     string
	)

	addUserCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new user",
		RunE: func(_ *cobra.Command, _ []string) error {
			if usrUsername == "" || usrPassword == "" || usrRole == "" {
				return fmt.Errorf("--username, --password and --role are required")
			}

			reqBody := map[string]string{
				"username": usrUsername,
				"password": usrPassword,
				"role":     usrRole,
			}

			b, err := json.Marshal(reqBody)
			if err != nil {
				return err
			}

			resp, err := doRequest(http.MethodPost, "users", b)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
			}

			fmt.Printf("User '%s' added successfully.\n", usrUsername)
			return nil
		},
	}

	editUserCmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a user",
		RunE: func(_ *cobra.Command, _ []string) error {
			if usrUsername == "" {
				return fmt.Errorf("--username is required")
			}

			reqBody := make(map[string]string)
			if usrPassword != "" {
				reqBody["password"] = usrPassword
			}
			if usrRole != "" {
				reqBody["role"] = usrRole
			}

			b, err := json.Marshal(reqBody)
			if err != nil {
				return err
			}

			resp, err := doRequest(http.MethodPut, fmt.Sprintf("users/%s", usrUsername), b)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
			}

			fmt.Printf("User '%s' edited successfully.\n", usrUsername)
			return nil
		},
	}

	deleteUserCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user",
		RunE: func(_ *cobra.Command, _ []string) error {
			if usrUsername == "" {
				return fmt.Errorf("--username is required")
			}

			resp, err := doRequest(http.MethodDelete, fmt.Sprintf("users/%s", usrUsername), nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
			}

			fmt.Printf("User '%s' deleted successfully.\n", usrUsername)
			return nil
		},
	}

	usersCmd.AddCommand(listUsersCmd)
	
	addUserCmd.Flags().StringVar(&usrUsername, "username", "", "Username")
	addUserCmd.Flags().StringVar(&usrPassword, "password", "", "Password")
	addUserCmd.Flags().StringVar(&usrRole, "role", "", "Role (admin, operator, viewer, service)")
	usersCmd.AddCommand(addUserCmd)

	editUserCmd.Flags().StringVar(&usrUsername, "username", "", "Username to edit")
	editUserCmd.Flags().StringVar(&usrPassword, "password", "", "New password")
	editUserCmd.Flags().StringVar(&usrRole, "role", "", "New role")
	usersCmd.AddCommand(editUserCmd)

	deleteUserCmd.Flags().StringVar(&usrUsername, "username", "", "Username to delete")
	usersCmd.AddCommand(deleteUserCmd)

	rootCmd.AddCommand(usersCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
