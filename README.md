# Запуск

- Для начала необходимо поднять поды. Это все было сделано в рамках [первой практической](https://github.com/AlanMute/five-gags).
- Далее нужно перебросить порты для каждой бдшки
```shell
kubectl port-forward svc/mongodb-service 27017:27017
kubectl port-forward svc/elasticsearch-service 9200:9200
kubectl port-forward svc/neo4j-service 7687:7687
kubectl port-forward svc/postgres-service 5432:5432
kubectl port-forward svc/redis-service 6379:6379
```
- После чего можно запускать сервис командой `go run .`

# Лабораторные

## №1
Выполнить запрос к структуре хранения информации о группах учащихся, курсах обучения, лекционной программе и составу лекционных курсов и практических занятий, а также структуре связей между курсами, специальностями, студентами кафедры и данными о посещении студентами занятий, для извлечения отчета о 10 студентах с минимальным процентом посещения лекция, содержащих заданный термин или фразу, за определенный период обучения. Состав полей должен включать Полную информацию о студенте, процент посещения, период отчета, термин в занятиях курса.

- Была создана ручка
```shell
GET http://localhost:8000/api/v1/attendance-report?term={{YOUR_TERM}}&startDate={{START_DATE}}&endDate={{END_DATE}}
```
- Данная ручка возвращает полную информацию о студентах следующего вида
```json
[
    {
        "student_id": "string",
        "name": "string",
        "group": "string",
        "course": "integer",
        "department": "string",
        "email": "string",
        "birth": "string",
        "attendance_rate": "integer",
        "reporting_period": "string",
        "matched_term": "string"
    }
]
```