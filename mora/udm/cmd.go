package udm

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type (
	udmCommand struct {
		client udmClient
	}

	udmCommandConfig struct {
		ServerAddr string `toml:"server"`
		RepoURL    string `toml:"repo"`
		Token      string `toml:"token"`
	}
)

func unpackMetricName(name string) (string, string, error) {
	a := strings.Split(name, "/")
	if len(a) != 2 {
		return "", "", fmt.Errorf("malformed metric name: %s", name)
	}

	return a[0], a[1], nil
}

func readConfigFile(filename string) (udmCommandConfig, error) {
	log.Print("readConfigFile")
	b, err := os.ReadFile(filename)
	if err != nil {
		return udmCommandConfig{}, err
	}

	var config udmCommandConfig
	if err := toml.Unmarshal(b, &config); err != nil {
		return udmCommandConfig{}, err
	}

	return config, nil
}

// ----------------------------------------------------------------------
// udmCommand: utils

func (c *udmCommand) resolveMetricByName(repoId int64, name string) (*metricModel, error) {
	metrics, err := c.client.listMetrics(repoId)
	if err != nil {
		return nil, err
	}

	for _, m := range metrics {
		if m.Name == name {
			return &m, nil
		}
	}

	return nil, errorMetricNotFound
}

func (c *udmCommand) resolveItemByName(repoId int64, metricId int64, name string) (*itemModel, error) {
	items, err := c.client.listItems(repoId, metricId)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.Name == name {
			return &item, nil
		}
	}

	return nil, errorItemNotFound
}

func (c *udmCommand) resolveMetric(repoId int64, name string) (*metricModel, *itemModel, error) {
	metricName, itemName, err := unpackMetricName(name)
	if err != nil {
		return nil, nil, err
	}

	metric, err := c.resolveMetricByName(repoId, metricName)
	if err != nil {
		return nil, nil, err
	}

	item, err := c.resolveItemByName(repoId, metric.Id, itemName)

	return metric, item, err
}

func (c *udmCommand) getRepoId(repoUrl string) (int64, error) {
	repos, err := c.client.listRepositories()
	if err != nil {
		return 0, err
	}

	for _, repo := range repos {
		// log.Print("url=", repo.Url)
		if repo.Url == repoUrl {
			return repo.Id, nil
		}
	}

	return 0, errorRepositoryNotFound
}

func (c *udmCommand) createMetric(repoId int64, name string, typ int) error {
	log.Print("udmCommand.createMetric: name=", name, " type=", typ)

	metricName, itemName, err := unpackMetricName(name)
	if err != nil {
		return err
	}

	metric, err := c.resolveMetricByName(repoId, metricName)
	if err == errorMetricNotFound {
		log.Print("udmCommand.createMetric: creating metric: ", metricName)
		metric = &metricModel{Name: metricName}
		if err = c.client.addMetric(repoId, metric); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	log.Print("udmCommand.createMetric: metric.Id=", metric.Id)

	item := itemModel{
		MetricId:  metric.Id,
		Name:      itemName,
		ValueType: typ,
	}

	return c.client.addItem(repoId, &item)
}

func (c *udmCommand) deleteMetric(repoId int64, name string) error {
	log.Print("udmCommand.deleteMetric: name=", name)
	metric, item, err := c.resolveMetric(repoId, name)
	if err != nil {
		return err
	}

	return c.client.deleteItem(repoId, metric.Id, item.Id)
}

func (c *udmCommand) listMetrics(repoId int64) error {
	metrics, err := c.client.listMetrics(repoId)
	if err != nil {
		return err
	}

	for _, m := range metrics {
		items, err := c.client.listItems(repoId, m.Id)
		if err != nil {
			return err
		}

		for _, item := range items {
			fmt.Printf("%4d %s/%s\n", m.Id, m.Name, item.Name)
		}
	}

	return nil
}

// ----------------------------------------------------------------------
// Value

func (c *udmCommand) addValue(
	repoId int64, name string, timestamp time.Time, value string) error {

	metric, item, err := c.resolveMetric(repoId, name)
	if err != nil {
		return err
	}

	revision := "" // FIXME

	val := &valueModel{
		ItemId:    item.Id,
		Revision:  revision,
		Timestamp: timestamp,
		Value:     value,
	}

	return c.client.addValue(repoId, metric.Id, val)
}

func (c *udmCommand) listValues(repoId int64, name string) error {
	metric, item, err := c.resolveMetric(repoId, name)
	if err != nil {
		return err
	}

	values, err := c.client.listValues(repoId, metric.Id, item.Id)
	if err != nil {
		return err
	}

	for _, v := range values {
		fmt.Printf("%s %s", v.Timestamp, v.Value)
	}

	return nil
}

func (c *udmCommand) deleteValues(repoId int64, name string) error {
	metric, item, err := c.resolveMetric(repoId, name)
	if err != nil {
		return err
	}

	return c.client.deleteValues(repoId, metric.Id, item.Id)
}

func processDebugOption(cmd *cobra.Command) {
	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

func (c *udmCommand) init(cmd *cobra.Command) (udmCommandConfig, error) {
	processDebugOption(cmd)

	filename, _ := cmd.Flags().GetString("config")
	var config udmCommandConfig
	if filename != "" {
		log.Print("filename=", filename)
		if _, err := os.Stat(filename); err == nil {
			config, err = readConfigFile(filename)
			if err != nil {
				return udmCommandConfig{}, err
			}
		}
	}

	// parse global flags

	if v, _ := cmd.Flags().GetString("server"); v != "" {
		config.ServerAddr = v
	}

	if v, _ := cmd.Flags().GetString("repo"); v != "" {
		config.ServerAddr = v
	}

	if v, _ := cmd.Flags().GetString("token"); v != "" {
		config.Token = v
	}

	key := os.Getenv("MORA_API_KEY")
	if key != "" {
		config.Token = key
	}

	c.client.init(config.ServerAddr, config.Token)

	return config, nil
}

func (c *udmCommand) runMetricCommand(cmd *cobra.Command, args []string) error {
	config, err := c.init(cmd)
	if err != nil {
		return err
	}

	repoId, err := c.getRepoId(config.RepoURL)
	if err != nil {
		return err
	}
	log.Print("udmCommand.runMetricCommand: repoId=", repoId)

	createCmd, _ := cmd.Flags().GetBool("create")
	deleteCmd, _ := cmd.Flags().GetBool("delete")
	listCmd, _ := cmd.Flags().GetBool("list")

	if createCmd {
		if len(args) != 1 {
			return errors.New("no metric name given")
		}
		typ, _ := cmd.Flags().GetString("type")
		if typ != "int" {
			return fmt.Errorf("unknown type: %s", typ)
		}
		cmd.SilenceUsage = true
		return c.createMetric(repoId, args[0], 1)
	} else if deleteCmd {
		if len(args) != 1 {
			return errors.New("no metric name given")
		}
		cmd.SilenceUsage = true
		return c.deleteMetric(repoId, args[0])
	} else if listCmd {
		if len(args) != 0 {
			return errors.New("unexpected args")
		}
		cmd.SilenceUsage = true
		return c.listMetrics(repoId)
	}

	return nil
}

func (c *udmCommand) runValueCommand(cmd *cobra.Command, args []string) error {
	config, err := c.init(cmd)
	if err != nil {
		return err
	}

	repoId, err := c.getRepoId(config.RepoURL)
	if err != nil {
		return err
	}

	clearFlag, _ := cmd.Flags().GetBool("clear")
	addFlag, _ := cmd.Flags().GetBool("add")
	listFlag, _ := cmd.Flags().GetBool("list")

	if clearFlag {
		if len(args) != 1 {
			return errors.New("no metric name give")
		}
		return c.deleteValues(repoId, args[0])
	} else if addFlag {
		if len(args) != 2 {
			return errors.New("no required args")
		}

		var timestamp time.Time
		timestamp_str, _ := cmd.Flags().GetString("time")
		log.Print("timestamp_str=", timestamp_str)
		if timestamp_str == "" {
			timestamp = time.Now()
		} else {
			var err error
			timestamp, err = time.Parse("2006-01-02", timestamp_str)
			if err != nil {
				return err
			}
			log.Print(timestamp)
		}

		return c.addValue(repoId, args[0], timestamp, args[1])
	} else if listFlag {
		if len(args) != 1 {
			return errors.New("no metric name give")
		}
		return c.listValues(repoId, args[0])
	}

	return nil
}

func (c *udmCommand) newMetricCommand() *cobra.Command {
	metricCmd := &cobra.Command{
		Use:   "metric [--create|-c]",
		Short: "metrict operations",
		Long:  "long",
		RunE:  c.runMetricCommand,
	}

	metricCmd.Flags().BoolP("create", "c", false, "create new metric")
	metricCmd.Flags().BoolP("delete", "d", false, "delete metric")
	metricCmd.Flags().BoolP("list", "l", false, "list metrics")
	metricCmd.Flags().String("type", "int", "metric type")

	return metricCmd
}

func (c *udmCommand) newValueCommand() *cobra.Command {
	valueCmd := &cobra.Command{
		Use:   "value [-clear | -create] [flags] metric [value]",
		Short: "value operations",
		RunE:  c.runValueCommand,
	}

	valueCmd.Flags().BoolP("add", "a", false, "add value")
	valueCmd.Flags().BoolP("list", "l", false, "list values")
	valueCmd.Flags().BoolP("clear", "c", false, "clear value")
	valueCmd.Flags().StringP("time", "t", "", "timestamp")

	return valueCmd
}

func (c *udmCommand) newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "udm",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("no sub command given")
		},
	}

	cmd.PersistentFlags().String("config", "", "config")
	cmd.PersistentFlags().StringP("server", "s", "", "server")
	cmd.PersistentFlags().StringP("repo", "r", "", "Url of repo")
	cmd.PersistentFlags().String("token", "", "token")
	cmd.PersistentFlags().Bool("debug", false, "debug log")

	cmd.AddCommand(c.newMetricCommand())
	cmd.AddCommand(c.newValueCommand())

	return cmd
}

func NewCommand() *cobra.Command {
	noColor := false
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: noColor}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	c := udmCommand{client: &udmClientImpl{client: &http.Client{}}}
	return c.newCommand()
}
