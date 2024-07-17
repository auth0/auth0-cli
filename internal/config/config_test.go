package config

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestDefaultPath(t *testing.T) {
	XDG_CONFIG_HOME, xdg_config_exists := os.LookupEnv("XDG_CONFIG_HOME")
	HOME, home_exists := os.LookupEnv("HOME")
	AUTH0_CONFIG_FILE, auth0_config_exists := os.LookupEnv("AUTH0_CONFIG_FILE")
	APPDATA, appdata_exists := os.LookupEnv("APPDATA")

	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("HOME")
	os.Unsetenv("AUTH0_CONFIG_FILE")
	os.Unsetenv("APPDATA")

	t.Cleanup(func() {
		if xdg_config_exists {
			os.Setenv("XDG_CONFIG_HOME", XDG_CONFIG_HOME)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
		if home_exists {
			os.Setenv("HOME", HOME)
		} else {
			os.Unsetenv("HOME")
		}
		if auth0_config_exists {
			os.Setenv("AUTH0_CONFIG_FILE", AUTH0_CONFIG_FILE)
		} else {
			os.Unsetenv("AUTH0_CONFIG_FILE")
		}
		if appdata_exists {
			os.Setenv("APPDATA", APPDATA)
		} else {
			os.Unsetenv("APPDATA")
		}
	})

	t.Run("it returns the path set in AUTH0_CONFIG_FILE", func(t *testing.T) {
		t.Setenv("AUTH0_CONFIG_FILE", "/path/to/auth0/config.json")
		t.Setenv("XDG_CONFIG_HOME", "/path/to/xdg_config_home")
		t.Setenv("HOME", "/path/to/home")
		t.Setenv("APPDATA", "/path/to/userprofile")
		expectedPath := "/path/to/auth0/config.json"
		actualPath := defaultPath()
		assert.Equal(t, expectedPath, actualPath)
	})

	t.Run("it returns the path set in XDG_CONFIG_HOME", PlatformTestXDGConfigHome)

	t.Run("it returns the path in the user config directory", PlatformTestConfigFileDefaultPath)
	t.Run("it fallsback to temp if no user config directory is found", func(t *testing.T) {
		os.Unsetenv("AUTH0_CONFIG_FILE")
		os.Unsetenv("XDG_CONFIG_HOME")
		os.Unsetenv("HOME")
		os.Unsetenv("USERPROFILE")
		os.Unsetenv("APPDATA")
		expectedPath := path.Join(os.TempDir(), "auth0", "config.json")
		actualPath := defaultPath()
		assert.Equal(t, expectedPath, actualPath)
	})
}

func TestConfig_LoadFromDisk(t *testing.T) {
	t.Run("it fails to load a non existent config file", func(t *testing.T) {
		config := &Config{path: "i-am-a-non-existent-config.json"}
		err := config.loadFromDisk()
		assert.EqualError(t, err, "config.json file is missing")
	})

	t.Run("it fails to load config if path is a directory", func(t *testing.T) {
		dirPath, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := os.Remove(dirPath)
			require.NoError(t, err)
		})

		config := &Config{path: dirPath}
		err = config.loadFromDisk()

		assert.EqualError(t, err, fmt.Sprintf("read %s: %s", dirPath, ErrFileIsADirectory))
	})

	t.Run("it fails to load an empty config file", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(""))

		config := &Config{path: tempFile}
		err := config.loadFromDisk()

		assert.EqualError(t, err, "unexpected end of JSON input")
	})

	t.Run("it can successfully load a config file with a logged in tenant", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`
		{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}
		`))

		expectedConfig := &Config{
			path:          tempFile,
			InstallID:     "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			DefaultTenant: "auth0-cli.eu.auth0.com",
			Tenants: Tenants{
				"auth0-cli.eu.auth0.com": Tenant{
					Name:        "auth0-cli",
					Domain:      "auth0-cli.eu.auth0.com",
					AccessToken: "eyfSaswe",
					ExpiresAt:   time.Date(2023, time.April, 18, 11, 18, 7, 998809000, time.UTC),
					ClientID:    "secret",
				},
			},
		}

		config := &Config{path: tempFile}
		err := config.loadFromDisk()

		assert.NoError(t, err)
		assert.Equal(t, expectedConfig, config)
	})

	t.Run("it can successfully load a config file with no logged in tenants", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`
		{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "",
			"tenants": {}
		}
		`))

		expectedConfig := &Config{
			path:      tempFile,
			InstallID: "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			Tenants:   map[string]Tenant{},
		}

		config := &Config{path: tempFile}
		err := config.loadFromDisk()

		assert.NoError(t, err)
		assert.Equal(t, expectedConfig, config)
	})
}

func TestConfig_SaveToDisk(t *testing.T) {
	var testCases = []struct {
		name           string
		config         *Config
		expectedOutput string
	}{
		{
			name: "valid config with a logged in tenant",
			config: &Config{
				InstallID:     "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
				DefaultTenant: "auth0-cli.eu.auth0.com",
				Tenants: Tenants{
					"auth0-cli.eu.auth0.com": Tenant{
						Name:        "auth0-cli",
						Domain:      "auth0-cli.eu.auth0.com",
						AccessToken: "eyfSaswe",
						ExpiresAt:   time.Date(2023, time.April, 18, 11, 18, 7, 998809000, time.UTC),
						ClientID:    "secret",
					},
				},
			},
			expectedOutput: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`,
		},
		{
			name: "valid config with no logged in tenants",
			config: &Config{
				InstallID: "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
				Tenants:   map[string]Tenant{},
			},
			expectedOutput: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "",
    "tenants": {}
}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			t.Cleanup(func() {
				err := os.RemoveAll(tmpDir)
				require.NoError(t, err)
			})

			testCase.config.path = path.Join(tmpDir, "auth0", "config.json")

			err = testCase.config.saveToDisk()
			assert.NoError(t, err)

			fileContent, err := os.ReadFile(testCase.config.path)
			assert.NoError(t, err)
			assert.Equal(t, string(fileContent), testCase.expectedOutput)
		})
	}

	t.Run("it fails to save config if file path is a read only directory", FailsToSaveToReadOnlyDirectory)
}

func TestConfig_GetTenant(t *testing.T) {
	t.Run("it can successfully retrieve a logged in tenant", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`
		{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}
		`))

		expectedTenant := Tenant{
			Name:        "auth0-cli",
			Domain:      "auth0-cli.eu.auth0.com",
			AccessToken: "eyfSaswe",
			ExpiresAt:   time.Date(2023, time.April, 18, 11, 18, 7, 998809000, time.UTC),
			ClientID:    "secret",
		}

		config := &Config{path: tempFile}
		actualTenant, err := config.GetTenant("auth0-cli.eu.auth0.com")

		assert.NoError(t, err)
		assert.Equal(t, expectedTenant, actualTenant)
	})

	t.Run("it throws an error if the tenant can't be found", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`
		{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "",
			"tenants": {}
		}
		`))

		config := &Config{path: tempFile}
		_, err := config.GetTenant("auth0-cli.eu.auth0.com")

		assert.EqualError(t, err, "failed to find tenant: auth0-cli.eu.auth0.com. Run 'auth0 tenants list' to see your configured tenants or run 'auth0 login' to configure a new tenant")
	})

	t.Run("it throws an error if the config can't be initialized", func(t *testing.T) {
		config := &Config{path: "non-existent-config.json"}
		_, err := config.GetTenant("auth0-cli.eu.auth0.com")

		assert.EqualError(t, err, "config.json file is missing")
	})
}

func TestConfig_AddTenant(t *testing.T) {
	t.Run("it can successfully add a tenant and create the config.json file", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			err := os.RemoveAll(tmpDir)
			require.NoError(t, err)
		})

		config := &Config{
			InstallID: "6122fd48-a634-447e-88b0-0580d41b7fb6",
			path:      path.Join(tmpDir, "auth0", "config.json"),
		}

		tenant := Tenant{
			Name:        "auth0-cli",
			Domain:      "auth0-cli.eu.auth0.com",
			AccessToken: "eyfSaswe",
			ExpiresAt:   time.Date(2023, time.April, 18, 11, 18, 7, 998809000, time.UTC),
			ClientID:    "secret",
		}

		err = config.AddTenant(tenant)
		assert.NoError(t, err)

		expectedOutput := `{
    "install_id": "6122fd48-a634-447e-88b0-0580d41b7fb6",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`

		assertConfigFileMatches(t, config.path, expectedOutput)
	})

	t.Run("it can successfully add another tenant to the config", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`
		{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}
		`))

		config := &Config{
			path: tempFile,
		}

		tenant := Tenant{
			Name:        "auth0-mega-cli",
			Domain:      "auth0-mega-cli.eu.auth0.com",
			AccessToken: "eyfSaswe",
			ExpiresAt:   time.Date(2023, time.April, 18, 11, 18, 7, 998809000, time.UTC),
			ClientID:    "secret",
		}

		err := config.AddTenant(tenant)
		assert.NoError(t, err)

		expectedOutput := `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        },
        "auth0-mega-cli.eu.auth0.com": {
            "name": "auth0-mega-cli",
            "domain": "auth0-mega-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`

		assertConfigFileMatches(t, config.path, expectedOutput)
	})
}

func TestConfig_RemoveTenant(t *testing.T) {
	var testCases = []struct {
		name           string
		givenConfig    string
		givenTenant    string
		expectedConfig string
	}{
		{
			name:        "it can successfully remove a tenant from the config",
			givenTenant: "auth0-mega-cli.eu.auth0.com",
			givenConfig: createTempConfigFile(t, []byte(`{
				"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
				"default_tenant": "auth0-cli.eu.auth0.com",
				"tenants": {
					"auth0-cli.eu.auth0.com": {
						"name": "auth0-cli",
						"domain": "auth0-cli.eu.auth0.com",
						"access_token": "eyfSaswe",
						"expires_at": "2023-04-18T11:18:07.998809Z",
						"client_id": "secret"
					},
					"auth0-mega-cli.eu.auth0.com": {
						"name": "auth0-mega-cli",
						"domain": "auth0-mega-cli.eu.auth0.com",
						"access_token": "eyfSaswe",
						"expires_at": "2023-04-18T11:18:07.998809Z",
						"client_id": "secret"
					}
				}
			}`)),
			expectedConfig: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`,
		},
		{
			name:        "it can successfully remove the default tenant from the config",
			givenTenant: "auth0-cli.eu.auth0.com",
			givenConfig: createTempConfigFile(t, []byte(`{
				"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
				"default_tenant": "auth0-cli.eu.auth0.com",
				"tenants": {
					"auth0-cli.eu.auth0.com": {
						"name": "auth0-cli",
						"domain": "auth0-cli.eu.auth0.com",
						"access_token": "eyfSaswe",
						"expires_at": "2023-04-18T11:18:07.998809Z",
						"client_id": "secret"
					},
					"auth0-mega-cli.eu.auth0.com": {
						"name": "auth0-mega-cli",
						"domain": "auth0-mega-cli.eu.auth0.com",
						"access_token": "eyfSaswe",
						"expires_at": "2023-04-18T11:18:07.998809Z",
						"client_id": "secret"
					}
				}
			}`)),
			expectedConfig: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-mega-cli.eu.auth0.com",
    "tenants": {
        "auth0-mega-cli.eu.auth0.com": {
            "name": "auth0-mega-cli",
            "domain": "auth0-mega-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`,
		},
		{
			name:        "it can successfully remove the last tenant from the config",
			givenTenant: "auth0-cli.eu.auth0.com",
			givenConfig: createTempConfigFile(t, []byte(`{
				"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
				"default_tenant": "auth0-cli.eu.auth0.com",
				"tenants": {
					"auth0-cli.eu.auth0.com": {
						"name": "auth0-cli",
						"domain": "auth0-cli.eu.auth0.com",
						"access_token": "eyfSaswe",
						"expires_at": "2023-04-18T11:18:07.998809Z",
						"client_id": "secret"
					}
				}
			}`)),
			expectedConfig: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "",
    "tenants": {}
}`,
		},
		{
			name:        "it doesn't do anything if config file has no logged in tenants",
			givenTenant: "auth0-cli.eu.auth0.com",
			givenConfig: createTempConfigFile(t, []byte(`{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "",
    "tenants": {}
}`)),
			expectedConfig: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "",
    "tenants": {}
}`,
		},
		{
			name:        "it sets the default tenant to empty if no logged in tenants are registered",
			givenTenant: "auth0-cli.eu.auth0.com",
			givenConfig: createTempConfigFile(t, []byte(`{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {}
}`)),
			expectedConfig: `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "",
    "tenants": {}
}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			config := &Config{path: testCase.givenConfig}

			err := config.RemoveTenant(testCase.givenTenant)
			assert.NoError(t, err)

			assertConfigFileMatches(t, config.path, testCase.expectedConfig)
		})
	}

	t.Run("it doesn't throw an error if config file is missing", func(t *testing.T) {
		config := &Config{
			path: "i-dont-exist.json",
		}

		err := config.RemoveTenant("auth0-cli.eu.auth0.com")
		assert.NoError(t, err)
	})
}

func TestConfig_ListAllTenants(t *testing.T) {
	t.Run("it can successfully list all tenants", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
				"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
				"default_tenant": "auth0-cli.eu.auth0.com",
				"tenants": {
					"auth0-cli.eu.auth0.com": {
						"name": "auth0-cli",
						"domain": "auth0-cli.eu.auth0.com",
						"access_token": "eyfSaswe",
						"expires_at": "2023-04-18T11:18:07.998809Z",
						"client_id": "secret"
					}
				}
			}`))

		expectedTenants := []Tenant{
			{
				Name:        "auth0-cli",
				Domain:      "auth0-cli.eu.auth0.com",
				AccessToken: "eyfSaswe",
				ExpiresAt:   time.Date(2023, time.April, 18, 11, 18, 7, 998809000, time.UTC),
				ClientID:    "secret",
			},
		}

		config := &Config{path: tempFile}
		actualTenants, err := config.ListAllTenants()

		assert.NoError(t, err)
		assert.Len(t, actualTenants, 1)
		assert.Equal(t, expectedTenants, actualTenants)
	})

	t.Run("it throws an error if there's an issue with the config file", func(t *testing.T) {
		config := &Config{path: "i-dont-exist.json"}

		_, err := config.ListAllTenants()
		assert.EqualError(t, err, "config.json file is missing")
	})
}

func TestConfig_SetDefaultTenant(t *testing.T) {
	t.Run("it can successfully save a new tenant default", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				},
				"auth0-mega-cli.eu.auth0.com": {
					"name": "auth0-mega-cli",
					"domain": "auth0-mega-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		expectedConfig := `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-mega-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        },
        "auth0-mega-cli.eu.auth0.com": {
            "name": "auth0-mega-cli",
            "domain": "auth0-mega-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`

		config := &Config{path: tempFile}
		err := config.SetDefaultTenant("auth0-mega-cli.eu.auth0.com")
		assert.NoError(t, err)
		assertConfigFileMatches(t, config.path, expectedConfig)
	})

	t.Run("it fails to save a new tenant default if it doesn't exist", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				},
				"auth0-mega-cli.eu.auth0.com": {
					"name": "auth0-mega-cli",
					"domain": "auth0-mega-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		config := &Config{path: tempFile}
		err := config.SetDefaultTenant("auth0-super-cli.eu.auth0.com")
		assert.EqualError(t, err, "failed to find tenant: auth0-super-cli.eu.auth0.com. Run 'auth0 tenants list' to see your configured tenants or run 'auth0 login' to configure a new tenant")
	})

	t.Run("it throws an error if there's an issue with the config file", func(t *testing.T) {
		config := &Config{path: "i-dont-exist.json"}

		err := config.SetDefaultTenant("tenant")
		assert.EqualError(t, err, "config.json file is missing")
	})
}

func TestConfig_SetDefaultAppIDForTenant(t *testing.T) {
	t.Run("it successfully saves a new default app id for the tenant", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		expectedConfig := `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "default_app_id": "appID123456",
            "client_id": "secret"
        }
    }
}`

		config := &Config{path: tempFile}
		err := config.SetDefaultAppIDForTenant("auth0-cli.eu.auth0.com", "appID123456")
		assert.NoError(t, err)
		assertConfigFileMatches(t, config.path, expectedConfig)
	})

	t.Run("it throws an error if there's an issue with the config file", func(t *testing.T) {
		config := &Config{path: "i-dont-exist.json"}

		err := config.SetDefaultAppIDForTenant("tenant", "appID123456")
		assert.EqualError(t, err, "config.json file is missing")
	})
}

func TestConfig_IsLoggedInWithTenant(t *testing.T) {
	t.Run("it returns true when there is a tenant in the config and its access token is valid", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6OTY4MTgzMzQ2MH0.DsEpQkL0MIWcGJOIfEY8vr3MVS_E0GYsachNLQwBu5Q",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		config := &Config{path: tempFile}
		assert.True(t, config.IsLoggedInWithTenant("auth0-cli.eu.auth0.com"))
	})

	t.Run("it returns false when the config file doesn't exist", func(t *testing.T) {
		config := &Config{path: "i-dont-exist.json"}
		assert.False(t, config.IsLoggedInWithTenant("auth0-cli.eu.auth0.com"))
	})

	t.Run("it returns false when there is a tenant in the config and its access token is expired", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6MTY4MTEzMzQ2MH0.dG481CD7v8VCzSsBHdApTiRDUuCZXBgk5LO__q4r2Fg",
					"expires_at": "2023-04-10T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		config := &Config{path: tempFile}
		assert.False(t, config.IsLoggedInWithTenant("auth0-cli.eu.auth0.com"))
	})

	t.Run("it returns false when there is a tenant in the config and its access token is malformed", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "dG481CD7v8VCzSsBHdApTiRDUuCZXBgk5LO__q4r2Fg",
					"expires_at": "2023-04-10T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		config := &Config{path: tempFile}
		assert.False(t, config.IsLoggedInWithTenant("auth0-cli.eu.auth0.com"))
	})

	t.Run("it returns true when there is a tenant in the config and its access token taken from the keyring is valid", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		keyring.MockInit()
		const secretAccessToken = "Auth0 CLI Access Token"
		const testTenantName = "auth0-cli.eu.auth0.com"
		err := keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 0), testTenantName, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.")
		assert.NoError(t, err)
		err = keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 1), testTenantName, "eyJpc3MiOiJodHRwczovL2F1dGgwLmF1dGgwLmNvbS8iLCJpYXQiOjE2ODExNDcwNjAsImV4cCI6OTY4MTgzMzQ2MH0.")
		assert.NoError(t, err)
		err = keyring.Set(fmt.Sprintf("%s %d", secretAccessToken, 2), testTenantName, "DsEpQkL0MIWcGJOIfEY8vr3MVS_E0GYsachNLQwBu5Q")
		assert.NoError(t, err)

		config := &Config{path: tempFile}
		assert.True(t, config.IsLoggedInWithTenant("auth0-cli.eu.auth0.com"))
	})
}

func TestConfig_Validate(t *testing.T) {
	t.Run("it successfully verifies that we are authenticated", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		config := &Config{path: tempFile}
		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("it throws an error if we are not authenticated with any tenant", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "auth0-cli.eu.auth0.com",
			"tenants": {}
		}`))

		config := &Config{path: tempFile}
		err := config.Validate()
		assert.EqualError(t, err, "not logged in. Try `auth0 login`")
	})

	t.Run("it fixes the default tenant if there are tenant entries and the default is empty", func(t *testing.T) {
		tempFile := createTempConfigFile(t, []byte(`{
			"install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
			"default_tenant": "",
			"tenants": {
				"auth0-cli.eu.auth0.com": {
					"name": "auth0-cli",
					"domain": "auth0-cli.eu.auth0.com",
					"access_token": "eyfSaswe",
					"expires_at": "2023-04-18T11:18:07.998809Z",
					"client_id": "secret"
				}
			}
		}`))

		config := &Config{path: tempFile}
		err := config.Validate()
		assert.NoError(t, err)

		expectedConfig := `{
    "install_id": "3998b053-dd7f-4bfe-bb10-c4f3a96a0180",
    "default_tenant": "auth0-cli.eu.auth0.com",
    "tenants": {
        "auth0-cli.eu.auth0.com": {
            "name": "auth0-cli",
            "domain": "auth0-cli.eu.auth0.com",
            "access_token": "eyfSaswe",
            "expires_at": "2023-04-18T11:18:07.998809Z",
            "client_id": "secret"
        }
    }
}`

		assertConfigFileMatches(t, config.path, expectedConfig)
	})

	t.Run("it throws an error if there's an issue with the config file", func(t *testing.T) {
		config := &Config{path: "i-dont-exist.json"}

		err := config.Validate()
		assert.EqualError(t, err, "config.json file is missing")
	})
}

func createTempConfigFile(t *testing.T, data []byte) string {
	t.Helper()

	tempFile, err := os.CreateTemp("", "config.json")
	require.NoError(t, err)

	t.Cleanup(func() {
		err := tempFile.Close()
		require.NoError(t, err)
		err = os.Remove(tempFile.Name())
		require.NoError(t, err)
	})

	_, err = tempFile.Write(data)
	require.NoError(t, err)

	return tempFile.Name()
}

func assertConfigFileMatches(t *testing.T, actualConfigPath, expectedConfig string) {
	t.Helper()

	fileContent, err := os.ReadFile(actualConfigPath)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, string(fileContent))
}
