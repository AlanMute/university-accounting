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
)
