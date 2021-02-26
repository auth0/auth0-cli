package cli

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/prompt"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"gopkg.in/auth0.v5/management"
)

var (
	//go:embed data/quickstarts.json
	qsBuf             []byte
	quickstartsByType = func() (qs map[string][]quickstart) {
		if err := json.Unmarshal(qsBuf, &qs); err != nil {
			panic(err)
		}
		return
	}()
)

type quickstart struct {
	Name    string   `json:"name"`
	Samples []string `json:"samples"`
	Org     string   `json:"org"`
	Repo    string   `json:"repo"`
	Branch  string   `json:"branch,omitempty"`
}

func quickstartsCmd(cli *cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "quickstarts",
		Short:   "Quickstart support for getting bootstrapped",
		Aliases: []string{"qs"},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.AddCommand(downloadQuickstart(cli))

	return cmd
}

func downloadQuickstart(cli *cli) *cobra.Command {
	var flags struct {
		ClientID string
		Type     string
		Stack    string
	}

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download a specific type and tech stack for quick starts",
		Long:  `auth0 quickstarts download --type <type> --client-id <client-id> --stack <stack>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := cli.api.Client.Read(flags.ClientID)
			if err != nil {
				return err
			}

			var selectedType = flags.Type
			var selectedStack = flags.Stack

			if selectedType == "" {
				input := prompt.SelectInput("type", "Quickstart Type:", "Type of quickstart to download", quickstartTypes(), true)
				if err := prompt.AskOne(input, &selectedType); err != nil {
					return err
				}
			}
			if selectedStack == "" {
				stacks, err := quickstartStacksFromType(selectedType)
				if err != nil {
					return err
				}
				input := prompt.SelectInput("stack", "Quickstart Stack:", "Tech/Language to use for quickstart", stacks, true)
				if err := prompt.AskOne(input, &selectedStack); err != nil {
					return err
				}
			}

			target, exists, err := quickstartPathFor(client)
			if err != nil {
				return err
			}

			if exists {
				// TODO(cyx): prompt for a warning to force overwrite.
				// For now, we're just exiting to simplify this first stab.
				cli.renderer.Warnf("WARNING: %s already exists. Run with --force to overwrite", target)
				return nil
			}

			q, err := getQuickstart(selectedType, selectedStack)
			if err != nil {
				return err
			}

			err = ansi.Spinner("Downloading quickstart", func() error {
				return downloadQuickStart(context.TODO(), cli, client, target, q)
			})

			if err != nil {
				return err
			}

			cli.renderer.Infof("Quickstart sucessfully downloaded at %s", target)
			return nil
		},
	}

	cmd.SetUsageTemplate(resourceUsageTemplate())
	cmd.Flags().StringVar(&flags.ClientID, "client-id", "", "ID of the client.")
	cmd.Flags().StringVarP(&flags.Type, "type", "t", "", "Type of the quickstart to download.")
	cmd.Flags().StringVarP(&flags.Stack, "stack", "s", "", "Tech stack of the quickstart to use.")
	mustRequireFlags(cmd, "client-id")

	return cmd
}

const (
	quickstartEndpoint           = `https://auth0.com/docs/package/v2`
	quickstartContentType        = `application/json`
	quickstartOrg                = "auth0-samples"
	quickstartDefaultCallbackURL = `https://YOUR_APP/callback`
)

func downloadQuickStart(ctx context.Context, cli *cli, client *management.Client, target string, q quickstart) error {
	var payload struct {
		Branch       string `json:"branch"`
		Org          string `json:"org"`
		Repo         string `json:"repo"`
		Path         string `json:"path"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		CallbackURL  string `json:"callback_url"`
		Domain       string `json:"domain"`
		Tenant       string `json:"tenant"`
	}

	ten, err := cli.getTenant()
	if err != nil {
		return err
	}

	payload.Tenant = ten.Name
	payload.Domain = ten.Domain

	// FIXME(copland): Default to first item from list of samples.
	// Eventually we should add a forced survey for user to select one if
	// there are multiple.
	payload.Branch = q.Branch
	payload.Repo = q.Repo
	payload.Path = q.Samples[0]

	fmt.Printf("%+v\n", payload)

	// These appear to be largely constant and refers to the github
	// username they're under.
	payload.Org = quickstartOrg
	payload.ClientID = client.GetClientID()
	payload.ClientSecret = client.GetClientSecret()

	// Callback URL, if not set, will just take the default one.
	payload.CallbackURL = quickstartDefaultCallbackURL
	if list := callbacksFor(client.Callbacks); len(list) > 0 {
		payload.CallbackURL = list[0]
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", quickstartEndpoint, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", quickstartContentType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	tmpfile, err := ioutil.TempFile("", "auth0-quickstart*.zip")
	if err != nil {
		return err
	}

	_, err = io.Copy(tmpfile, res.Body)
	if err != nil {
		return err
	}

	if err := tmpfile.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	if err := archiver.Unarchive(tmpfile.Name(), target); err != nil {
		return err
	}

	return nil
}

func quickstartPathFor(client *management.Client) (p string, exists bool, err error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	re := regexp.MustCompile(`[^\w]+`)
	friendlyName := re.ReplaceAllString(client.GetName(), "-")
	target := path.Join(wd, friendlyName)

	exists = true
	if _, err := os.Stat(target); err != nil {
		if !os.IsNotExist(err) {
			return "", false, err
		}
		exists = false
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return "", false, err
	}

	return target, exists, nil
}

func getQuickstart(typ, stack string) (quickstart, error) {
	quickstarts, ok := quickstartsByType[typ]
	if !ok {
		return quickstart{}, fmt.Errorf("unknown quickstart type %s", typ)
	}
	for _, q := range quickstarts {
		if q.Name == stack {
			return q, nil
		}
	}
	return quickstart{}, fmt.Errorf("quickstart not found for %s/%s", typ, stack)
}

func quickstartTypes() []string {
	keys := make([]string, 0, len(quickstartsByType))
	for k := range quickstartsByType {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func quickstartStacksFromType(t string) ([]string, error) {
	_, ok := quickstartsByType[t]
	if !ok {
		return nil, fmt.Errorf("unknown quickstart type %s", t)
	}
	stacks := make([]string, 0, len(quickstartsByType[t]))
	for _, s := range quickstartsByType[t] {
		stacks = append(stacks, s.Name)
	}
	return stacks, nil
}
