package cli

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/display"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/auth0.v5/management"
)

func TestCustomDomainsCmd(t *testing.T) {
	t.Run("List", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockCustomDomainAPI(ctrl)
		m.EXPECT().List().MaxTimes(1).Return([]*management.CustomDomain{{ID: auth0.String("testID"), Domain: auth0.String("testDomain"), Type: auth0.String("testType")}}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{CustomDomain: m},
		}

		cmd := customDomainsListCmd(cli)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		assert.JSONEq(t, `[{"ID": "testID", "Domain": "testDomain", "Type": "testType", "Primary": false, "Status": "", "VerificationMethod": "","Verification": {"Methods": null}}]`, stdout.String())
	})

	t.Run("Create", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockCustomDomainAPI(ctrl)
		m.EXPECT().Create(gomock.Any()).MaxTimes(1).Return(nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{CustomDomain: m},
		}

		cmd := customDomainsCreateCmd(cli)
		cmd.SetArgs([]string{"--domain=testDomain", "--type=testType", "--verification-method=testVerificationMethod"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		assert.JSONEq(t, `[{"ID": "", "Domain": "testDomain", "Type": "testType", "Primary": false, "Status": "", "VerificationMethod": "testVerificationMethod", "Verification": {"Methods": null}}]`, stdout.String())
	})

	t.Run("Delete", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockCustomDomainAPI(ctrl)
		m.EXPECT().Delete(gomock.Any()).MaxTimes(1).Return(nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{CustomDomain: m},
		}

		cmd := customDomainsDeleteCmd(cli)
		cmd.SetArgs([]string{"--custom-domain-id=testCustomDomainID"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockCustomDomainAPI(ctrl)
		m.EXPECT().Read(gomock.Any()).MaxTimes(1).Return(&management.CustomDomain{ID: auth0.String("testID"), Domain: auth0.String("testDomain"), Type: auth0.String("testType")}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{CustomDomain: m},
		}

		cmd := customDomainsGetCmd(cli)
		cmd.SetArgs([]string{"--custom-domain-id=testCustomDomainID"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		assert.JSONEq(t, `[{"ID": "testID", "Domain": "testDomain", "Type": "testType", "Primary": false, "Status": "", "VerificationMethod": "", "Verification": {"Methods": null}}]`, stdout.String())
	})

	t.Run("Verify", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := auth0.NewMockCustomDomainAPI(ctrl)
		m.EXPECT().Verify(gomock.Any()).MaxTimes(1).Return(&management.CustomDomain{ID: auth0.String("testID"), Domain: auth0.String("testDomain"), Type: auth0.String("testType")}, nil)
		stdout := &bytes.Buffer{}
		cli := &cli{
			renderer: &display.Renderer{
				MessageWriter: ioutil.Discard,
				ResultWriter:  stdout,
				Format:        display.OutputFormat("json"),
			},
			api: &auth0.API{CustomDomain: m},
		}

		cmd := customDomainsVerifyCmd(cli)
		cmd.SetArgs([]string{"--custom-domain-id=testCustomDomainID"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		assert.JSONEq(t, `[{"ID": "testID", "Domain": "testDomain", "Type": "testType", "Primary": false, "Status": "", "VerificationMethod": "", "Verification": {"Methods": null}}]`, stdout.String())
	})
}
