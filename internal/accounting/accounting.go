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

type CourseReport struct {
	DisciplineName        string        `json:"discipline_name"`
	DisciplineDescription string        `json:"discipline_description"`
	Lectures              []LectureInfo `json:"lectures"`
}

type LectureInfo struct {
	Topic          string   `json:"topic"`
	Type           string   `json:"type"`
	Date           string   `json:"date"`
	StudentCount   int      `json:"student_count"`
	TechEquipments []string `json:"tech_equipments"`
}

func (c *Client) GenerateCourseReport(year, semester int) ([]CourseReport, error) {
	var reports []CourseReport = make([]CourseReport, 0)
	var startDate, endDate string
	if semester == 1 {
		startDate = fmt.Sprintf("%d-09-01", year)
		endDate = fmt.Sprintf("%d-12-31", year)
	} else {
		startDate = fmt.Sprintf("%d-03-01", year+1)
		endDate = fmt.Sprintf("%d-08-31", year+1)
	}

	disciplineIDs, err := c.getDisciplinesForDateRange(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get disciplines: %v", err)
	}

	if len(disciplineIDs) == 0 {
		return reports, nil
	}

	disciplineData, err := c.getDisciplinesFromElastic(disciplineIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get discipline details: %v", err)
	}

	for _, discipline := range disciplineData {
		disciplineID := discipline["discipline_id"].(string)

		lectures, err := c.getLecturesWithDetails(disciplineID, startDate, endDate)

		if err != nil {
			return nil, fmt.Errorf("failed to get lectures for discipline %d: %v", disciplineID, err)
		}

		reports = append(reports, CourseReport{
			DisciplineName:        discipline["name"].(string),
			DisciplineDescription: discipline["description"].(string),
			Lectures:              lectures,
		})
	}

	return reports, nil
}

func (c *Client) getDisciplinesForDateRange(startDate, endDate string) ([]int, error) {
	rows, err := c.pgdbClient.Query(getDisciplinesForDateQuery, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query disciplines: %v", err)
	}
	defer rows.Close()

	var disciplineIDs []int
	for rows.Next() {
		var disciplineID int
		if err := rows.Scan(&disciplineID); err != nil {
			return nil, fmt.Errorf("failed to scan discipline_id: %v", err)
		}
		disciplineIDs = append(disciplineIDs, disciplineID)
	}

	return disciplineIDs, nil
}

var (
	typeToStringLesson = map[int]string{
		1: "Лекция",
		2: "Практика",
		3: "Лабораторная",
	}
)

func (c *Client) getDisciplinesFromElastic(disciplineIDs []int) ([]map[string]interface{}, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"terms": map[string]interface{}{
				"discipline_id": disciplineIDs,
			},
		},
	}

	var result map[string]interface{}

	res, err := c.esClient.Search(
		c.esClient.Search.WithIndex("disciplines"),
		c.esClient.Search.WithBody(strings.NewReader(mustJSON(query))),
		c.esClient.Search.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search disciplines: %v", err)
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode ElasticSearch response: %v", err)
	}

	hits := result["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) == 0 {
		return nil, fmt.Errorf("no disciplines found")
	}

	var disciplines []map[string]interface{}
	for _, hit := range hits {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		disciplines = append(disciplines, source)
	}

	return disciplines, nil
}

func (c *Client) getLecturesWithDetails(disciplineID, startDate, endDate string) ([]LectureInfo, error) {
	rows, err := c.pgdbClient.Query(getLecturesWithDetailsQuery, disciplineID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query lectures: %v", err)
	}
	defer rows.Close()

	var lectures []LectureInfo
	for rows.Next() {
		var lecture LectureInfo
		var typeLecture int
		var techEquipments []sql.NullString

		if err := rows.Scan(&lecture.Topic, &typeLecture, &lecture.Date, &lecture.StudentCount, pq.Array(&techEquipments)); err != nil {
			return nil, fmt.Errorf("failed to scan lecture row: %v", err)
		}

		equipmentsSet := make(map[string]struct{})
		for _, eq := range techEquipments {
			if eq.Valid {
				equipmentsSet[eq.String] = struct{}{}
			}
		}

		for equipment := range equipmentsSet {
			lecture.TechEquipments = append(lecture.TechEquipments, equipment)
		}

		lecture.Type = typeToStringLesson[typeLecture]

		lectures = append(lectures, lecture)
	}

	return lectures, nil
}

type GroupReport struct {
	GroupName string        `json:"group_name"`
	Students  []StudentInfo `json:"students"`
}

type StudentInfo struct {
	StudentID   string             `json:"student_id"`
	Name        string             `json:"name"`
	Group       string             `json:"group"`
	Course      int                `json:"course"`
	Email       string             `json:"email"`
	Birth       string             `json:"birth"`
	Disciplines []DisciplineReport `json:"disciplines"`
}

type DisciplineReport struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	PlannedHours  int    `json:"planned_hours"`
	AttendedHours int    `json:"attended_hours"`
}

func (c *Client) GenerateGroupReport(groupName string) (*GroupReport, error) {
	groupID, studentIDs, err := c.getGroupAndStudentsByName(groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get group and students: %v", err)
	}

	students, err := c.getStudentsInfoFromRedis(studentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get students info from Redis: %v", err)
	}

	disciplineIDs, err := c.getSpecialDisciplinesForGroup(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get special disciplines: %v", err)
	}

	for i, student := range students {
		for _, disciplineID := range disciplineIDs {
			discipline, err := c.getDisciplineFromElastic(disciplineID)
			if err != nil {
				return nil, fmt.Errorf("failed to get discipline description: %v", err)
			}

			plannedHours, attendedHours, err := c.calculateHours(groupID, student.StudentID, disciplineID)
			if err != nil {
				return nil, fmt.Errorf("failed to calculate hours: %v", err)
			}

			student.Disciplines = append(student.Disciplines, DisciplineReport{
				Name:          discipline["name"].(string),
				Description:   discipline["description"].(string),
				PlannedHours:  plannedHours,
				AttendedHours: attendedHours,
			})
		}
		students[i] = student
	}

	return &GroupReport{
		GroupName: groupName,
		Students:  students,
	}, nil
}

func (c *Client) getSpecialDisciplinesForGroup(groupID int) ([]int, error) {
	rows, err := c.pgdbClient.Query(getSpecialDisciplinesQuery, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query special disciplines: %v", err)
	}
	defer rows.Close()

	var disciplineIDs []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan discipline_id: %v", err)
		}
		disciplineIDs = append(disciplineIDs, id)
	}

	return disciplineIDs, nil
}

func (c *Client) getStudentsInfoFromRedis(studentIDs []string) ([]StudentInfo, error) {
	var students []StudentInfo

	for _, studentID := range studentIDs {
		key := fmt.Sprintf("student:%s", studentID)
		data, err := c.redisClient.Get(context.Background(), key).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get student data for %s: %v", studentID, err)
		}

		var student StudentInfo
		if err := json.Unmarshal([]byte(data), &student); err != nil {
			return nil, fmt.Errorf("failed to unmarshal student data: %v", err)
		}

		students = append(students, student)
	}

	return students, nil
}

func (c *Client) getGroupAndStudentsByName(groupName string) (int, []string, error) {
	rows, err := c.pgdbClient.Query(getGroupAndStudentsByNameQuery, groupName)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to query group and students: %v", err)
	}
	defer rows.Close()

	var groupID int
	var studentIDs []string
	for rows.Next() {
		var cardID string
		if err := rows.Scan(&groupID, &cardID); err != nil {
			return 0, nil, fmt.Errorf("failed to scan row: %v", err)
		}
		studentIDs = append(studentIDs, cardID)
	}

	return groupID, studentIDs, nil
}

func (c *Client) getDisciplineFromElastic(disciplineID int) (map[string]interface{}, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"discipline_id": disciplineID,
			},
		},
	}

	var result map[string]interface{}
	res, err := c.esClient.Search(
		c.esClient.Search.WithIndex("disciplines"),
		c.esClient.Search.WithBody(strings.NewReader(mustJSON(query))),
		c.esClient.Search.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search discipline: %v", err)
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	hits := result["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) == 0 {
		return nil, fmt.Errorf("discipline not found")
	}

	return hits[0].(map[string]interface{})["_source"].(map[string]interface{}), nil
}

func (c *Client) calculateHours(groupID int, studentID string, disciplineID int) (int, int, error) {
	var plannedHours, attendedHours int

	if err := c.pgdbClient.QueryRow(plannedQuery, disciplineID, groupID).Scan(&plannedHours); err != nil {
		return 0, 0, fmt.Errorf("failed to get planned hours: %v", err)
	}

	if err := c.pgdbClient.QueryRow(attendedQuery, disciplineID, groupID, studentID).Scan(&attendedHours); err != nil {
		return 0, 0, fmt.Errorf("failed to get attended hours: %v", err)
	}

	return plannedHours, attendedHours, nil
}

func (c *Client) GetAllGroups() ([]string, error) {
	rows, err := c.pgdbClient.Query(getAllGroupsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query group and students: %v", err)
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var group string
		if err := rows.Scan(&group); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		groups = append(groups, group)
	}

	return groups, nil
}
