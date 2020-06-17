package spanner

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/spanner"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
)

const queryStatsTopMinute = `
SELECT
  text,
  text_truncated,
  text_fingerprint,
  interval_end,
  execution_count,
  avg_latency_seconds,
  avg_rows,
  avg_bytes,
  avg_rows_scanned,
  avg_cpu_seconds
FROM {{.Table}}
`

const (
	queryStatsTopMinuteTable   = "spanner_sys.query_stats_top_minute"
	queryStatsTop10MinuteTable = "spanner_sys.query_stats_top_10minute"
	queryStatsTopHourTable     = "spanner_sys.query_stats_top_hour"
)

type QueryStatsParam struct {
	Table string
}

type QueryStatsCopyService struct {
	queryStatsTopQueryTemplate *template.Template
	spanner                    *spanner.Client
	bq                         *bigquery.Client
}

func NewQueryStatsCopyService(ctx context.Context, spannerClient *spanner.Client, bqClient *bigquery.Client) (*QueryStatsCopyService, error) {
	tmpl, err := template.New("getQueryStatsTopQuery").Parse(queryStatsTopMinute)
	if err != nil {
		return nil, err
	}

	return &QueryStatsCopyService{
		queryStatsTopQueryTemplate: tmpl,
		spanner:                    spannerClient,
		bq:                         bqClient,
	}, nil
}

type QueryStat struct {
	InsertID          string
	IntervalEnd       time.Time `spanner:"interval_end"` // End of the time interval that the included query executions occurred in.
	Text              string    // SQL query text, truncated to approximately 64KB.
	TextTruncated     bool      `spanner:"text_truncated"`      // Whether or not the query text was truncated.
	TextFingerprint   int64     `spanner:"text_fingerprint"`    // Hash of the query text.
	ExecuteCount      int64     `spanner:"execution_count"`     // Number of times Cloud Spanner saw the query during the interval.
	AvgLatencySeconds float64   `spanner:"avg_latency_seconds"` // Average length of time, in seconds, for each query execution within the database. This average excludes the encoding and transmission time for the result set as well as overhead.
	AvgRows           float64   `spanner:"avg_rows"`            // Average number of rows that the query returned.
	AvgBytes          float64   `spanner:"avg_bytes"`           // Average number of data bytes that the query returned, excluding transmission encoding overhead.
	AvgRowsScanned    float64   `spanner:"avg_rows_scanned"`    // Average number of rows that the query scanned, excluding deleted values.
	AvgCPUSeconds     float64   `spanner:"avg_cpu_seconds"`     // Average number of seconds of CPU time Cloud Spanner spent on all operations to execute the query.
}

func (s *QueryStat) ToInsertID() string {
	s.InsertID = fmt.Sprintf("%v-_-%v", s.IntervalEnd.Unix(), s.TextFingerprint)
	return s.InsertID
}

func (s *QueryStatsCopyService) GetQueryStats(ctx context.Context, table string) ([]*QueryStat, error) {
	var tpl bytes.Buffer
	if err := s.queryStatsTopQueryTemplate.Execute(&tpl, QueryStatsParam{Table: table}); err != nil {
		return nil, err
	}
	iter := s.spanner.Single().Query(ctx, spanner.NewStatement(tpl.String()))
	defer iter.Stop()

	rets := []*QueryStat{}
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}

		var result QueryStat
		if err := row.ToStruct(&result); err != nil {
			return nil, errors.WithStack(err)
		}
		rets = append(rets, &result)
	}

	return rets, nil
}

var queryStatsBigQueryTableSchema = bigquery.Schema{
	{Name: "IntervalEnd", Required: true, Type: bigquery.TimestampFieldType},
	{Name: "Text", Required: true, Type: bigquery.StringFieldType},
	{Name: "TextTruncated", Required: true, Type: bigquery.BooleanFieldType},
	{Name: "TextFingerprint", Required: true, Type: bigquery.IntegerFieldType},
	{Name: "ExecuteCount", Required: true, Type: bigquery.IntegerFieldType},
	{Name: "AvgLatencySeconds", Required: true, Type: bigquery.FloatFieldType},
	{Name: "AvgRows", Required: true, Type: bigquery.FloatFieldType},
	{Name: "AvgBytes", Required: true, Type: bigquery.FloatFieldType},
	{Name: "AvgRowsScanned", Required: true, Type: bigquery.FloatFieldType},
	{Name: "AvgCPUSeconds", Required: true, Type: bigquery.FloatFieldType},
}

func (s *QueryStatsCopyService) ToBigQuery(ctx context.Context, dataset *bigquery.Dataset, table string, qss []*QueryStat) error {
	var sss []*bigquery.StructSaver
	for _, qs := range qss {
		insertID := qs.ToInsertID()
		sss = append(sss, &bigquery.StructSaver{
			Schema:   queryStatsBigQueryTableSchema,
			InsertID: insertID,
			Struct:   qs,
		})
	}

	if err := s.bq.DatasetInProject(dataset.ProjectID, dataset.DatasetID).Table(table).Inserter().Put(ctx, sss); err != nil {
		return err
	}
	return nil
}