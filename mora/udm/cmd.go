package udm

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

type (
	cli struct {
		client udmClient
	}
)

func unpackMetricName(name string) (string, string, error) {
	a := strings.Split(name, "/")
	if len(a) != 2 {
		return "", "", fmt.Errorf("malformed metric name: %s", name)
	}

	return a[0], a[1], nil
}

// ----------------------------------------------------------------------
// cli

func (cli *cli) getRepoId(repoUrl string) (int64, error) {
	repos, err := cli.client.listRepositories()
	if err != nil {
		return 0, err
	}

	for _, repo := range repos {
		// log.Print("url=", repo.Url)
		if repo.Url == repoUrl {
			return repo.Id, nil
		}
	}

	return 0, fmt.Errorf("repository not found: %s", repoUrl)
}

func (cli *cli) findMetric(repoId int64, name string) (*metricModel, error) {
	metrics, err := cli.client.listMetrics(repoId)
	if err != nil {
		return nil, err
	}

	log.Print("cli.findMetric: len(metrics)=", len(metrics))

	for _, m := range metrics {
		if m.Name == name {
			return &m, nil
		}
	}

	return nil, nil
}

func (cli *cli) findItem(repoId int64, metricId int64, name string) (*itemModel, error) {
	items, err := cli.client.listItems(repoId, metricId)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.Name == name {
			return &item, nil
		}
	}

	return nil, fmt.Errorf("no item: %s", name)
}

func (cli *cli) createMetric(repoId int64, name string, typ int) error {
	log.Print("cli.createMetric: name=", name, " type=", typ)

	metricName, itemName, err := unpackMetricName(name)
	if err != nil {
		return err
	}

	metric, err := cli.findMetric(repoId, metricName)
	if err != nil {
		return err
	}

	if metric == nil {
		log.Print("cli.createMetric: creating mttric: ", metricName)
		metric = &metricModel{Name: metricName}
		err = cli.client.addMetric(repoId, metric)
		if err != nil {
			return err
		}
	}

	log.Print("cli.createMetric: metric.Id=", metric.Id)

	item := itemModel{
		MetricId:  metric.Id,
		Name:      itemName,
		ValueType: typ,
	}

	return cli.client.addItem(repoId, &item)
}

func (cli *cli) deleteMetric(repoId int64, name string) error {
	log.Print("cli.deleteMetric: name=", name)
	metricName, itemName, err := unpackMetricName(name)
	if err != nil {
		return err
	}

	metric, err := cli.findMetric(repoId, metricName)
	if err != nil {
		return err
	}

	if metric == nil {
		return errors.New("no metric found")
	}

	item, err := cli.findItem(repoId, metric.Id, itemName)
	if err != nil {
		return err
	}

	return cli.client.deleteItem(repoId, metric.Id, item.Id)
}

func (cli *cli) listMetrics(repoId int64) error {
	metrics, err := cli.client.listMetrics(repoId)
	if err != nil {
		return err
	}

	for _, m := range metrics {
		items, err := cli.client.listItems(repoId, m.Id)
		if err != nil {
			return err
		}

		for _, item := range items {
			fmt.Printf("%4d %s/%s\n", m.Id, m.Name, item.Name)
		}
	}

	return nil
}

func (cli *cli) pushValue(
	repoId int64, name string, timestamp time.Time, value string) error {

	metricName, itemName, err := unpackMetricName(name)
	if err != nil {
		return err
	}

	metric, err := cli.findMetric(repoId, metricName)
	if err != nil {
		return err
	}

	if metric == nil {
		return errors.New("no metric found")
	}

	item, err := cli.findItem(repoId, metric.Id, itemName)
	if err != nil {
		return err
	}

	if item == nil {
		return errors.New("no item found")
	}

	revision := "" // FIXME

	val := &valueModel{
		ItemId:    item.Id,
		Revision:  revision,
		Timestamp: timestamp,
		Value:     value,
	}

	return cli.client.addValue(repoId, metric.Id, val)
}

// ----------------------------------------------------------------------
// value

type (
	Args struct {
		serverAddr string
		repoUrl    string
		token      string
	}
)

func parseArgs(cmd *cobra.Command) Args {
	server, _ := cmd.Flags().GetString("server")
	repo, _ := cmd.Flags().GetString("repo")
	token, _ := cmd.Flags().GetString("token")

	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return Args{
		serverAddr: server,
		repoUrl:    repo,
		token:      token,
	}
}

func (cli *cli) runMetricCommand(cmd *cobra.Command, args []string) error {
	log.Print("runMetricCommand")
	log.Print("client=", cli.client)

	parsedArgs := parseArgs(cmd)

	cli.client.init(parsedArgs.serverAddr, parsedArgs.token)
	repoId, err := cli.getRepoId(parsedArgs.repoUrl)
	if err != nil {
		return err
	}
	log.Print("cli.runMetricCommand: repoId=", repoId)

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
		return cli.createMetric(repoId, args[0], 1)
	} else if deleteCmd {
		if len(args) != 1 {
			return errors.New("no metric name given")
		}
		cmd.SilenceUsage = true
		return cli.deleteMetric(repoId, args[0])
	} else if listCmd {
		if len(args) != 0 {
			return errors.New("unexpected args")
		}
		cmd.SilenceUsage = true
		return cli.listMetrics(repoId)
	}

	return nil
}

func (cli *cli) runPushCommand(cmd *cobra.Command, args []string) error {
	log.Print("runMetricCommand")

	parsedArgs := parseArgs(cmd)
	metric, _ := cmd.Flags().GetString("metric")

	var timestamp time.Time
	timestamp_str, _ := cmd.Flags().GetString("time")
	if timestamp_str == "" {
		timestamp = time.Now()
	} else {
		var err error
		timestamp, err = time.Parse("2006-01-02", timestamp_str)
		if err != nil {
			return err
		}
	}

	cli.client.init(parsedArgs.serverAddr, parsedArgs.token)
	repoId, err := cli.getRepoId(parsedArgs.repoUrl)
	if err != nil {
		return err
	}
	log.Print("cli.runPushCommand: repoId=", repoId)

	if len(args) != 1 {
		return errors.New("no value given")
	}

	return cli.pushValue(repoId, metric, timestamp, args[0])
}

func (cli *cli) newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "udm",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("no sub command given")
		},
	}

	metricCmd := &cobra.Command{
		Use:   "metric",
		Short: "metrict operations",
		RunE:  cli.runMetricCommand,
	}

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "push value",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.runPushCommand(cmd, args)
		},
	}

	cmd.PersistentFlags().StringP("server", "s", "", "server")
	cmd.PersistentFlags().StringP("repo", "r", "", "Url of repo")
	cmd.PersistentFlags().StringP("token", "t", "token", "token")
	cmd.PersistentFlags().Bool("debug", false, "debug log")

	metricCmd.Flags().BoolP("create", "c", false, "Create new metric")
	metricCmd.Flags().BoolP("delete", "d", false, "Delete metric")
	metricCmd.Flags().BoolP("list", "l", false, "List metrics")
	metricCmd.Flags().String("type", "int", "Metric type")

	pushCmd.Flags().StringP("metric", "m", "", "metric")
	pushCmd.Flags().String("time", "", "timestamp")

	cmd.AddCommand(metricCmd)
	cmd.AddCommand(pushCmd)

	return cmd
}

func NewCommand() *cobra.Command {
	noColor := false
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: noColor}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	cli := cli{client: &udmClientImpl{client: &http.Client{}}}
	return cli.newCommand()
}
