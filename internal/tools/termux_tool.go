package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/terbash/terbash/pkg/types"
)

type TermuxTool struct{}

func NewTermuxTool() *TermuxTool {
	return &TermuxTool{}
}

func (t *TermuxTool) Name() string {
	return "termux_api"
}

func (t *TermuxTool) Description() string {
	return "Access Termux:API features (battery, clipboard, notification, wifi, etc.)"
}

func (t *TermuxTool) Schema() types.ToolSchema {
	return types.ToolSchema{
		Type: "object",
		Properties: map[string]types.Property{
			"operation": {
				Type:        "string",
				Description: "Termux API operation",
				Enum: []string{
					"battery", "clipboard_get", "clipboard_set",
					"notification", "toast", "vibrate",
					"wifi_scan", "wifi_info", "sensor",
					"location", "contact_list", "sms_list",
					"telephony_deviceinfo", "camera_info",
				},
			},
			"args": {
				Type:        "object",
				Description: "Operation-specific arguments",
			},
		},
		Required: []string{"operation"},
	}
}

func (t *TermuxTool) Execute(args map[string]interface{}) (*types.ToolResult, error) {
	op, _ := args["operation"].(string)
	if op == "" {
		return &types.ToolResult{Success: false, Error: "operation is required"}, nil
	}

	argMap, _ := args["args"].(map[string]interface{})

	var cmd *exec.Cmd
	switch op {
	case "battery":
		cmd = exec.Command("termux-battery-status")
	case "clipboard_get":
		cmd = exec.Command("termux-clipboard-get")
	case "clipboard_set":
		text, _ := argMap["text"].(string)
		if text == "" {
			return &types.ToolResult{Success: false, Error: "text required for clipboard_set"}, nil
		}
		cmd = exec.Command("termux-clipboard-set", text)
	case "notification":
		title, _ := argMap["title"].(string)
		content, _ := argMap["content"].(string)
		if title == "" || content == "" {
			return &types.ToolResult{Success: false, Error: "title and content required"}, nil
		}
		cmd = exec.Command("termux-notification", "--title", title, "--content", content)
	case "toast":
		text, _ := argMap["text"].(string)
		if text == "" {
			return &types.ToolResult{Success: false, Error: "text required for toast"}, nil
		}
		cmd = exec.Command("termux-toast", text)
	case "vibrate":
		duration := "100"
		if d, ok := argMap["duration"].(string); ok {
			duration = d
		}
		cmd = exec.Command("termux-vibrate", "-d", duration)
	case "wifi_scan":
		cmd = exec.Command("termux-wifi-scaninfo")
	case "wifi_info":
		cmd = exec.Command("termux-wifi-connectioninfo")
	case "sensor":
		sensorType, _ := argMap["type"].(string)
		if sensorType == "" {
			sensorType = "all"
		}
		cmd = exec.Command("termux-sensor", "-t", sensorType)
	case "location":
		provider, _ := argMap["provider"].(string)
		if provider == "" {
			provider = "gps"
		}
		cmd = exec.Command("termux-location", "-p", provider)
	case "contact_list":
		cmd = exec.Command("termux-contact-list")
	case "sms_list":
		limit := "10"
		if l, ok := argMap["limit"].(string); ok {
			limit = l
		}
		cmd = exec.Command("termux-sms-list", "-l", limit)
	case "telephony_deviceinfo":
		cmd = exec.Command("termux-telephony-deviceinfo")
	case "camera_info":
		cmd = exec.Command("termux-camera-info")
	default:
		return &types.ToolResult{Success: false, Error: fmt.Sprintf("unknown operation: %s", op)}, nil
	}

	output, err := cmd.CombinedOutput()
	return &types.ToolResult{
		Success: err == nil,
		Output:  strings.TrimSpace(string(output)),
		Error:   errToString(err),
		Metadata: map[string]string{"operation": op},
	}, nil
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}