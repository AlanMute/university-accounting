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
)
