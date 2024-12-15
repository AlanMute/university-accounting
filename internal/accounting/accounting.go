package accounting

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"sort"
	"strconv"
	"strings"
)

type Client struct {
	redisClient *redis.Client
	mongoClient *mongo.Client
	neoClient   neo4j.Driver
	pgdbClient  *sql.DB
	esClient    *elasticsearch.Client
}

func NewClient(redisClient *redis.Client, mongoClient *mongo.Client, neoClient neo4j.Driver, pgdbClient *sql.DB, esClient *elasticsearch.Client) *Client {
	return &Client{
		redisClient: redisClient,
		mongoClient: mongoClient,
		neoClient:   neoClient,
		pgdbClient:  pgdbClient,
		esClient:    esClient,
	}
}

type StudentReport struct {
	StudentID       string  `json:"student_id"`
	Name            string  `json:"name"`
	Group           string  `json:"group"`
	Course          int     `json:"course"`
	Department      string  `json:"department"`
	Email           string  `json:"email"`
	Birth           string  `json:"birth"`
	AttendanceRate  float64 `json:"attendance_rate"`
	ReportingPeriod string  `json:"reporting_period"`
	MatchedTerm     string  `json:"matched_term"`
}

func (c *Client) GenerateAttendanceReport(term string, startDate, endDate string) ([]StudentReport, error) {
	ctx := context.Background()

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_phrase": map[string]interface{}{
				"content": term,
			},
		},
	}
	var esResult map[string]interface{}

	esRes, err := c.esClient.Search(
		c.esClient.Search.WithContext(ctx),
		c.esClient.Search.WithIndex("materials"),
		c.esClient.Search.WithBody(strings.NewReader(mustJSON(query))),
		c.esClient.Search.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query ElasticSearch: %v", err)
	}
	defer esRes.Body.Close()

	if err := json.NewDecoder(esRes.Body).Decode(&esResult); err != nil {
		return nil, fmt.Errorf("failed to decode ElasticSearch response: %v", err)
	}

	matchingMaterials := extractMatchingMaterialIDs(esResult)
	matchingLectures, err := c.getLecturesByMaterials(matchingMaterials)
	if err != nil {
		return nil, fmt.Errorf("failed to get lectures by materials: %v", err)
	}

	attendanceData, err := c.getAttendanceData(matchingLectures, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get attendance data: %v", err)
	}

	var reports []StudentReport
	for studentID, attendanceRate := range attendanceData {
		student, err := c.getStudentDetails(ctx, studentID)
		if err != nil {
			logrus.Errorf("failed to get student details for ID %s: %v", studentID, err)
			continue
		}

		reports = append(reports, StudentReport{
			StudentID:       studentID,
			Name:            student["name"].(string),
			Group:           student["group"].(string),
			Course:          int(student["course"].(float64)),
			Department:      student["department-name"].(string),
			Email:           student["email"].(string),
			Birth:           student["birth"].(string),
			AttendanceRate:  attendanceRate,
			ReportingPeriod: fmt.Sprintf("%s to %s", startDate, endDate),
			MatchedTerm:     term,
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].AttendanceRate < reports[j].AttendanceRate
	})

	if len(reports) > 10 {
		reports = reports[:10]
	}

	return reports, nil
}

func (c *Client) getAttendanceData(lectureIDs []int64, startDate, endDate string) (map[string]float64, error) {
	rows, err := c.pgdbClient.Query(getAttendanceDataQuery, pq.Array(lectureIDs), startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to getAttendanceDataQuery PostgreSQL: %v", err)
	}
	defer rows.Close()

	attendanceData := make(map[string]float64)
	for rows.Next() {
		var studentID string
		var attendanceRate float64
		if err := rows.Scan(&studentID, &attendanceRate); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		attendanceData[studentID] = attendanceRate
	}
	return attendanceData, nil
}

func (c *Client) getStudentDetails(ctx context.Context, studentID string) (map[string]interface{}, error) {
	data, err := c.redisClient.Get(ctx, fmt.Sprintf("student:%s", studentID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get student from Redis: %v", err)
	}

	var student map[string]interface{}
	if err := json.Unmarshal([]byte(data), &student); err != nil {
		return nil, fmt.Errorf("failed to unmarshal student data: %v", err)
	}
	return student, nil
}

func (c *Client) getLecturesByMaterials(materialIDs []int) ([]int64, error) {
	session := c.neoClient.NewSession(neo4j.SessionConfig{})
	defer session.Close()

	query :=
		`MATCH (m:Material)-[:MAT_LES]->(l:Lesson)
	WHERE m.id IN $materialIDs
	RETURN l.id AS lessonID`

	params := map[string]interface{}{
		"materialIDs": materialIDs,
	}

	result, err := session.Run(query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query Neo4j: %v", err)
	}

	var lectureIDs []int64
	for result.Next() {
		lectureIDs = append(lectureIDs, result.Record().GetByIndex(0).(int64))
	}

	return lectureIDs, nil
}

func extractMatchingMaterialIDs(esResult map[string]interface{}) []int {
	var materialIDs []int
	hits := esResult["hits"].(map[string]interface{})["hits"].([]interface{})
	for _, hit := range hits {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		if materialIDStr, ok := source["material_id"].(string); ok {
			if materialID, err := strconv.Atoi(materialIDStr); err == nil {
				materialIDs = append(materialIDs, materialID)
			}
		}
	}
	return materialIDs
}

func mustJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
