package accounting

const (
	getAttendanceDataQuery = `
		SELECT s.card_id,
       COUNT(CASE WHEN a.status = true THEN 1 END)::float / COUNT(*) AS attendance_rate
		FROM attendance a
		JOIN student s ON a.student_id = s.student_id
		JOIN schedule sch ON a.schedule_id = sch.schedule_id
		WHERE sch.lesson_id = ANY($1)
		  AND sch.date BETWEEN $2 AND $3
		  AND s.group_id = sch.group_id
		GROUP BY s.card_id
		ORDER BY attendance_rate ASC
		LIMIT 10;
	`

	getDisciplinesForDateQuery = `
		SELECT DISTINCT l.discipline_id
		FROM lesson l
		JOIN schedule sch ON l.lesson_id = sch.lesson_id
		WHERE sch.date BETWEEN $1 AND $2;
	`

	getLecturesWithDetailsQuery = `
		SELECT l.topic, l.type, sch.date, 
		       COUNT(a.student_id) AS student_count,
		       array_agg(e.name) AS tech_equipments
		FROM lesson l
		JOIN schedule sch ON l.lesson_id = sch.lesson_id
		LEFT JOIN attendance a ON sch.schedule_id = a.schedule_id
		LEFT JOIN equipment_requirements er ON l.lesson_id = er.lesson_id
		LEFT JOIN equipment e ON er.equipment = e.id
		WHERE l.discipline_id = $1 AND sch.date BETWEEN $2 AND $3
		GROUP BY l.lesson_id, sch.date
		ORDER BY sch.date;
	`

	getSpecialDisciplinesQuery = `
		SELECT DISTINCT c.discipline_id
		FROM course c
		JOIN lesson l ON c.discipline_id = l.discipline_id
		JOIN schedule sch ON l.lesson_id = sch.lesson_id
		WHERE c.is_special = true AND sch.group_id = $1;
	`

	getGroupAndStudentsByNameQuery = `
		SELECT g.group_id, s.card_id
		FROM "group" g
		JOIN student s ON g.group_id = s.group_id
		WHERE g.name = $1;
	`

	plannedQuery = `
		SELECT COUNT(*) * 2 AS planned_hours
		FROM schedule sch
		JOIN lesson l ON sch.lesson_id = l.lesson_id
		WHERE l.discipline_id = $1 AND sch.group_id = $2;
	`

	attendedQuery = `
		SELECT COUNT(*) * 2 AS attended_hours
		FROM attendance a
		JOIN schedule sch ON a.schedule_id = sch.schedule_id
		JOIN lesson l ON sch.lesson_id = l.lesson_id
		WHERE l.discipline_id = $1 AND sch.group_id = $2
		  AND a.status = true AND a.student_id = (
			SELECT student_id FROM student WHERE card_id = $3
		  );
	`

	getAllGroupsQuery = "SELECT name FROM \"group\""
)
