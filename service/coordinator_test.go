package service

import (
	"fmt"
	"github.com/adamringhede/influxdb-ha/service/merge"
	"github.com/influxdata/influxdb/influxql"
	"github.com/influxdata/influxdb/models"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_buildTree(t *testing.T) {
	stmt := mustGetSelect(`SELECT top("foo", 2), mean("value") FROM sales where time < now() group by time(1h) `)
	_, _, err := merge.NewQueryTree(stmt)
	assert.NoError(t, err)
}

func Test_mergeResults(t *testing.T) {

	a := Result{Series: []*models.Row{
		{
			Columns: []string{"time", "sum_value_", "top_value__1_", "mean_value_", "count_value_"},
			Values: [][]interface{}{
				{"1970-01-01T00:00:00Z", 5.0, 5.0, 5.0, 1},
				{"1970-01-02T00:00:00Z", 8.0, 8.0, 8.0, 1},
			}},
	}}

	b := Result{Series: []*models.Row{
		{
			Columns: []string{"time", "sum_value_", "top_value__1_", "mean_value_", "count_value_"},
			Values: [][]interface{}{
				{"1970-01-01T00:00:00Z", 3.0, 3.0, 3.0, 1},
				{"1970-01-02T00:00:00Z", 12.0, 12.0, 12.0, 1},
			}},
	}}

	// The results have toe be grouped by tags
	source := NewResultSource([]Result{a, b})

	assert.Equal(t, []float64{5, 3}, source.Next("sum_value_"))
	source.Step()
	assert.Equal(t, []float64{8, 12}, source.Next("sum_value_"))
	source.Reset()

	stmt := mustGetSelect(`SELECT mean("value"), top("value", 1), sum("value"), mean("value") * 3 FROM sales where time < now() group by time(1d) `)
	tree, _, err := merge.NewQueryTree(stmt)
	assert.NoError(t, err)
	assert.Equal(t, []float64{4}, tree.Fields[0].Root.Next(source))
	assert.Equal(t, []float64{5}, tree.Fields[1].Root.Next(source))
	assert.Equal(t, []float64{8}, tree.Fields[2].Root.Next(source))
	assert.Equal(t, []float64{12}, tree.Fields[3].Root.Next(source))
	source.Step()
	assert.Equal(t, []float64{10}, tree.Fields[0].Root.Next(source))
	assert.Equal(t, []float64{12}, tree.Fields[1].Root.Next(source))
	assert.Equal(t, []float64{20}, tree.Fields[2].Root.Next(source))
	assert.Equal(t, []float64{30}, tree.Fields[3].Root.Next(source))
	source.Reset()
}

func mustGetSelect(q string) *influxql.SelectStatement {
	query, err := influxql.ParseQuery(q)
	if err != nil {
		panic(err)
	}
	stmts := getSelectStatements(query.Statements)
	if len(stmts) != 1 {
		panic(fmt.Errorf("Expected to receive 1 statement from query. Got %d", len(stmts)))
	}
	return stmts[0]
}

func getSelectStatements(statements influxql.Statements) (result []*influxql.SelectStatement) {
	for _, stmt := range statements {
		switch s := stmt.(type) {
		case *influxql.SelectStatement:
			result = append(result, s)
		}
	}
	return result
}


/*
TODO Testing
Need two or three instances,
Test when an instance is unreachable (the randomness need to be seeded)
	For instance, the resolver should be able to select one and it should have a parameter for a seed.
 */