/*
Copyright 2021 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	"testing"
	"time"

	"vitess.io/vitess/go/test/endtoend/cluster"

	"vitess.io/vitess/go/test/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"vitess.io/vitess/go/mysql"
	"vitess.io/vitess/go/sqltypes"
)

// AssertMatches ensures the given query produces the expected results.
func AssertMatches(t testing.TB, conn *mysql.Conn, query, expected string) {
	t.Helper()
	qr := Exec(t, conn, query)
	got := fmt.Sprintf("%v", qr.Rows)
	diff := cmp.Diff(expected, got)
	if diff != "" {
		t.Errorf("Query: %s (-want +got):\n%s\nGot:%s", query, diff, got)
	}
}

// AssertMatchesAny ensures the given query produces any one of the expected results.
func AssertMatchesAny(t testing.TB, conn *mysql.Conn, query string, expected ...string) {
	t.Helper()
	qr := Exec(t, conn, query)
	got := fmt.Sprintf("%v", qr.Rows)
	for _, e := range expected {
		diff := cmp.Diff(e, got)
		if diff == "" {
			return
		}
	}
	t.Errorf("Query: %s (-want +got):\n%v\nGot:%s", query, expected, got)
}

// AssertMatchesCompareMySQL executes the given query on both Vitess and MySQL and make sure
// they have the same result set. The result set of Vitess is then matched with the given expectation.
func AssertMatchesCompareMySQL(t *testing.T, vtConn, mysqlConn *mysql.Conn, query, expected string) {
	t.Helper()
	qr := ExecCompareMySQL(t, vtConn, mysqlConn, query)
	got := fmt.Sprintf("%v", qr.Rows)
	diff := cmp.Diff(expected, got)
	if diff != "" {
		t.Errorf("Query: %s (-want +got):\n%s\nGot:%s", query, diff, got)
	}
}

// AssertContainsError ensures that the given query returns a certain error.
func AssertContainsError(t *testing.T, conn *mysql.Conn, query, expected string) {
	t.Helper()
	_, err := ExecAllowError(t, conn, query)
	require.Error(t, err)
	assert.Contains(t, err.Error(), expected, "actual error: %s", err.Error())
}

// AssertMatchesNoOrder executes the given query and makes sure it matches the given `expected` string.
// The order applied to the results or expectation is ignored. They are both re-sorted.
func AssertMatchesNoOrder(t *testing.T, conn *mysql.Conn, query, expected string) {
	t.Helper()
	qr := Exec(t, conn, query)
	actual := fmt.Sprintf("%v", qr.Rows)
	assert.Equal(t, utils.SortString(expected), utils.SortString(actual), "for query: [%s] expected \n%s \nbut actual \n%s", query, expected, actual)
}

// AssertIsEmpty ensures that the given query returns 0 row.
func AssertIsEmpty(t *testing.T, conn *mysql.Conn, query string) {
	t.Helper()
	qr := Exec(t, conn, query)
	assert.Empty(t, qr.Rows, "for query: "+query)
}

func AssertSingleRowIsReturned(t *testing.T, conn *mysql.Conn, predicate string, expectedKs string) {
	t.Run(predicate, func(t *testing.T) {
		qr, err := conn.ExecuteFetch("SELECT distinct table_schema FROM information_schema.tables WHERE "+predicate, 1000, true)
		require.NoError(t, err)
		assert.Equal(t, 1, len(qr.Rows), "did not get enough rows back")
		assert.Equal(t, expectedKs, qr.Rows[0][0].ToString())
	})
}

func AssertResultIsEmpty(t *testing.T, conn *mysql.Conn, pre string) {
	t.Run(pre, func(t *testing.T) {
		qr, err := conn.ExecuteFetch("SELECT distinct table_schema FROM information_schema.tables WHERE "+pre, 1000, true)
		require.NoError(t, err)
		assert.Empty(t, qr.Rows)
	})
}

// Exec executes the given query using the given connection. The results are returned.
// The test fails if the query produces an error.
func Exec(t testing.TB, conn *mysql.Conn, query string) *sqltypes.Result {
	t.Helper()
	qr, err := conn.ExecuteFetch(query, 1000, true)
	require.NoError(t, err, "for query: "+query)
	return qr
}

// ExecCompareMySQL executes the given query against both Vitess and MySQL and compares
// the two result set. If there is a mismatch, the difference will be printed and the
// test will fail. If the query produces an error in either Vitess or MySQL, the test
// will be marked as failed.
// The result set of Vitess is returned to the caller.
func ExecCompareMySQL(t *testing.T, vtConn, mysqlConn *mysql.Conn, query string) *sqltypes.Result {
	t.Helper()
	vtQr, err := vtConn.ExecuteFetch(query, 1000, true)
	require.NoError(t, err, "[Vitess Error] for query: "+query)

	mysqlQr, err := mysqlConn.ExecuteFetch(query, 1000, true)
	require.NoError(t, err, "[MySQL Error] for query: "+query)
	compareVitessAndMySQLResults(t, query, vtQr, mysqlQr, false)
	return vtQr
}

// ExecAllowError executes the given query without failing the test if it produces
// an error. The error is returned to the client, along with the result set.
func ExecAllowError(t *testing.T, conn *mysql.Conn, query string) (*sqltypes.Result, error) {
	t.Helper()
	return conn.ExecuteFetch(query, 1000, true)
}

// SkipIfBinaryIsBelowVersion skips the given test if the binary's major version is below majorVersion.
func SkipIfBinaryIsBelowVersion(t *testing.T, majorVersion int, binary string) {
	version, err := cluster.GetMajorVersion(binary)
	if err != nil {
		return
	}
	if version < majorVersion {
		t.Skip("Current version of ", binary, ": v", version, ", expected version >= v", majorVersion)
	}
}

// SkipIfBinaryIsAboveVersion skips the given test if the binary's major version is above majorVersion.
func SkipIfBinaryIsAboveVersion(t *testing.T, majorVersion int, binary string) {
	version, err := cluster.GetMajorVersion(binary)
	if err != nil {
		return
	}
	if version > majorVersion {
		t.Skip("Current version of ", binary, ": v", version, ", expected version >= v", majorVersion)
	}
}

// AssertMatchesWithTimeout asserts that the given query produces the expected result.
// The query will be executed every 'r' duration until it matches the expected result.
// If after 'd' duration we still did not find the expected result, the test will be marked as failed.
func AssertMatchesWithTimeout(t *testing.T, conn *mysql.Conn, query, expected string, r time.Duration, d time.Duration, failureMsg string) {
	t.Helper()
	timeout := time.After(d)
	diff := "actual and expectation does not match"
	for len(diff) > 0 {
		select {
		case <-timeout:
			require.Fail(t, failureMsg, diff)
		case <-time.After(r):
			qr := Exec(t, conn, query)
			diff = cmp.Diff(expected,
				fmt.Sprintf("%v", qr.Rows))
		}

	}
}

// WaitForAuthoritative waits for a table to become authoritative
func WaitForAuthoritative(t *testing.T, vtgateProcess cluster.VtgateProcess, ks, tbl string) error {
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("schema tracking didn't mark table t2 as authoritative until timeout")
		default:
			time.Sleep(1 * time.Second)
			res, err := vtgateProcess.ReadVSchema()
			require.NoError(t, err, res)
			t2Map := getTableT2Map(res, ks, tbl)
			authoritative, fieldPresent := t2Map["column_list_authoritative"]
			if !fieldPresent {
				continue
			}
			authoritativeBool, isBool := authoritative.(bool)
			if !isBool || !authoritativeBool {
				continue
			}
			return nil
		}
	}
}

func getTableT2Map(res *interface{}, ks, tbl string) map[string]interface{} {
	step1 := convertToMap(*res)["keyspaces"]
	step2 := convertToMap(step1)[ks]
	step3 := convertToMap(step2)["tables"]
	tblMap := convertToMap(step3)[tbl]
	return convertToMap(tblMap)
}

func convertToMap(input interface{}) map[string]interface{} {
	output := input.(map[string]interface{})
	return output
}
