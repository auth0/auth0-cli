package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/auth0/auth0-cli/internal/validators"
	"github.com/spf13/viper"
)

// Profile handles all things related to managing the project specific configurations
type Profile struct {
	DeviceName          string
	ProfileName         string
	APIKey              string
	TerminalPOSDeviceID string
}

// CreateProfile creates a profile when logging in
func (p *Profile) CreateProfile() error {
	writeErr := p.writeProfile(viper.GetViper())
	if writeErr != nil {
		return writeErr
	}

	return nil
}

// GetColor gets the color setting for the user based on the flag or the
// persisted color stored in the config file
func (p *Profile) GetColor() (string, error) {
	color := viper.GetString("color")
	if color != "" {
		return color, nil
	}

	color = viper.GetString(p.GetConfigField("color"))
	switch color {
	case "", ColorAuto:
		return ColorAuto, nil
	case ColorOn:
		return ColorOn, nil
	case ColorOff:
		return ColorOff, nil
	default:
		return "", fmt.Errorf("color value not supported: %s", color)
	}
}

// GetDeviceName returns the configured device name
func (p *Profile) GetDeviceName() (string, error) {
	if os.Getenv("AUTH0_DEVICE_NAME") != "" {
		return os.Getenv("AUTH0_DEVICE_NAME"), nil
	}

	if p.DeviceName != "" {
		return p.DeviceName, nil
	}

	if err := viper.ReadInConfig(); err == nil {
		return viper.GetString(p.GetConfigField("device_name")), nil
	}

	return "", errors.New("your device name has not been configured. Use `auth0 login` to set your device name")
}

// GetAPIKey will return the existing key for the given profile
func (p *Profile) GetAPIKey() (string, error) {
	envKey := os.Getenv("AUTH0_API_KEY")
	if envKey != "" {
		err := validators.APIKey(envKey)
		if err != nil {
			return "", err
		}

		return envKey, nil
	}

	if p.APIKey != "" {
		err := validators.APIKey(p.APIKey)
		if err != nil {
			return "", err
		}

		return p.APIKey, nil
	}

	// Try to fetch the API key from the configuration file
	if err := viper.ReadInConfig(); err == nil {
		key := viper.GetString(p.GetConfigField("api_key"))

		err := validators.APIKey(key)
		if err != nil {
			return "", err
		}

		return key, nil
	}

	return "", errors.New("your API key has not been configured. Use `auth0 login` to set your API key")
}

// GetTerminalPOSDeviceID returns the device id from the config for Terminal quickstart to use
func (p *Profile) GetTerminalPOSDeviceID() string {
	if err := viper.ReadInConfig(); err == nil {
		return viper.GetString(p.GetConfigField("terminal_pos_device_id"))
	}

	return ""
}

// GetConfigField returns the configuration field for the specific profile
func (p *Profile) GetConfigField(field string) string {
	return p.ProfileName + "." + field
}

// RegisterAlias registers an alias for a given key.
func (p *Profile) RegisterAlias(alias, key string) {
	viper.RegisterAlias(p.GetConfigField(alias), p.GetConfigField(key))
}

// WriteConfigField updates a configuration field and writes the updated
// configuration to disk.
func (p *Profile) WriteConfigField(field, value string) error {
	viper.Set(p.GetConfigField(field), value)
	return viper.WriteConfig()
}

// DeleteConfigField deletes a configuration field.
func (p *Profile) DeleteConfigField(field string) error {
	v, err := removeKey(viper.GetViper(), p.GetConfigField(field))
	if err != nil {
		return err
	}

	return p.writeProfile(v)
}

func (p *Profile) writeProfile(runtimeViper *viper.Viper) error {
	profilesFile := viper.ConfigFileUsed()

	err := makePath(profilesFile)
	if err != nil {
		return err
	}

	if p.DeviceName != "" {
		runtimeViper.Set(p.GetConfigField("device_name"), strings.TrimSpace(p.DeviceName))
	}

	if err := runtimeViper.MergeInConfig(); err != nil {
		log.Println(err)
	}

	runtimeViper.SetConfigFile(profilesFile)

	// Ensure we preserve the config file type
	runtimeViper.SetConfigType(filepath.Ext(profilesFile))

	err = runtimeViper.WriteConfig()
	if err != nil {
		return err
	}

	return nil
}
