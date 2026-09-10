package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)


type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
}

type Student struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Class    string `json:"class"`
	Age      int    `json:"age"`
	Email    string `json:"email"`
	UserID   int    `json:"user_id"`
}

type Teacher struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Subject struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	TeacherID int    `json:"teacher_id"`
}

type Grade struct {
	ID        int    `json:"id"`
	StudentID int    `json:"student_id"`
	SubjectID int    `json:"subject_id"`
	Value     int    `json:"value"`
	Date      string `json:"date"`
	Comment   string `json:"comment"`
}

type Homework struct {
	ID          int    `json:"id"`
	SubjectID   int    `json:"subject_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IssuedDate  string `json:"issued_date"`
	Deadline    string `json:"deadline"` 
}

type ErrorResponse struct {
	Error string `json:"error"`
}



type DB struct {
	mu        sync.Mutex
	users     map[int]User
	students  map[int]Student
	teachers  map[int]Teacher
	subjects  map[int]Subject
	grades    map[int]Grade
	homeworks map[int]Homework
	tokens    map[string]int

	userSeq  int
	studSeq  int
	teachSeq int
	subjSeq  int
	gradeSeq int
	hwSeq    int
	tokenSeq int
}

var db = &DB{
	users:     make(map[int]User),
	students:  make(map[int]Student),
	teachers:  make(map[int]Teacher),
	subjects:  make(map[int]Subject),
	grades:    make(map[int]Grade),
	homeworks: make(map[int]Homework),
	tokens:    make(map[string]int),
}


func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func generateSimpleToken() string {
	db.tokenSeq++
	return fmt.Sprintf("token-%d-%d", time.Now().UnixNano(), db.tokenSeq)
}

func parseID(pathValue string) (int, bool) {
	var id int
	_, err := fmt.Sscanf(pathValue, "%d", &id)
	if err != nil {
		return 0, false
	}
	return id, true
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "Missing token")
			return
		}

		db.mu.Lock()
		_, exists := db.tokens[token]
		db.mu.Unlock()

		if !exists {
			writeError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		next(w, r)
	}
}


func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	for _, u := range db.users {
		if u.Email == req.Email {
			writeError(w, http.StatusConflict, "Email already registered")
			return
		}
	}

	db.userSeq++
	user := User{ID: db.userSeq, Email: req.Email, PasswordHash: req.Password}
	db.users[user.ID] = user

	writeJSON(w, http.StatusCreated, map[string]int{"id": user.ID})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	var found *User
	for _, u := range db.users {
		if u.Email == req.Email && u.PasswordHash == req.Password {
			found = &u
			break
		}
	}

	if found == nil {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	token := generateSimpleToken()
	db.tokens[token] = found.ID

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	db.mu.Lock()
	userID := db.tokens[token]
	user := db.users[userID]
	db.mu.Unlock()
	writeJSON(w, http.StatusOK, user)
}


func handleStudents(w http.ResponseWriter, r *http.Request) {
	db.mu.Lock()
	defer db.mu.Unlock()

	classFilter := r.URL.Query().Get("class")
	var res []Student
	for _, s := range db.students {
		if classFilter == "" || s.Class == classFilter {
			res = append(res, s)
		}
	}
	if res == nil {
		res = []Student{}
	}
	writeJSON(w, http.StatusOK, res)
}

func handleCreateStudent(w http.ResponseWriter, r *http.Request) {
	var s Student
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil || s.FullName == "" || s.Class == "" {
		writeError(w, http.StatusBadRequest, "Invalid student data")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.studSeq++
	s.ID = db.studSeq
	db.students[s.ID] = s
	writeJSON(w, http.StatusCreated, s)
}

func handleStudentItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	s, exists := db.students[id]
	if !exists {
		writeError(w, http.StatusNotFound, "Student not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s)
	case http.MethodPut, http.MethodPatch:
		var updated Student
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid body")
			return
		}
		updated.ID = id
		db.students[id] = updated
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		delete(db.students, id)
		w.WriteHeader(http.StatusOK)
	}
}

func handleTeachers(w http.ResponseWriter, r *http.Request) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var res []Teacher
	for _, t := range db.teachers {
		res = append(res, t)
	}
	if res == nil {
		res = []Teacher{}
	}
	writeJSON(w, http.StatusOK, res)
}

func handleCreateTeacher(w http.ResponseWriter, r *http.Request) {
	var t Teacher
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil || t.Name == "" {
		writeError(w, http.StatusBadRequest, "Invalid teacher data")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.teachSeq++
	t.ID = db.teachSeq
	db.teachers[t.ID] = t
	writeJSON(w, http.StatusCreated, t)
}

func handleTeacherItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	t, exists := db.teachers[id]
	if !exists {
		writeError(w, http.StatusNotFound, "Teacher not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, t)
	case http.MethodPut, http.MethodPatch:
		var updated Teacher
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid body")
			return
		}
		updated.ID = id
		db.teachers[id] = updated
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		delete(db.teachers, id)
		w.WriteHeader(http.StatusOK)
	}
}


func handleSubjects(w http.ResponseWriter, r *http.Request) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var res []Subject
	for _, s := range db.subjects {
		res = append(res, s)
	}
	if res == nil {
		res = []Subject{}
	}
	writeJSON(w, http.StatusOK, res)
}

func handleCreateSubject(w http.ResponseWriter, r *http.Request) {
	var sub Subject
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil || sub.Title == "" {
		writeError(w, http.StatusBadRequest, "Invalid subject data")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, ok := db.teachers[sub.TeacherID]; !ok {
		writeError(w, http.StatusBadRequest, "Teacher not found")
		return
	}

	db.subjSeq++
	sub.ID = db.subjSeq
	db.subjects[sub.ID] = sub
	writeJSON(w, http.StatusCreated, sub)
}

func handleSubjectItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	sub, exists := db.subjects[id]
	if !exists {
		writeError(w, http.StatusNotFound, "Subject not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, sub)
	case http.MethodPut, http.MethodPatch:
		var updated Subject
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid body")
			return
		}
		if _, ok := db.teachers[updated.TeacherID]; !ok {
			writeError(w, http.StatusBadRequest, "Teacher not found")
			return
		}
		updated.ID = id
		db.subjects[id] = updated
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		delete(db.subjects, id)
		w.WriteHeader(http.StatusOK)
	}
}


func handleGrades(w http.ResponseWriter, r *http.Request) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var res []Grade
	for _, g := range db.grades {
		res = append(res, g)
	}
	if res == nil {
		res = []Grade{}
	}
	writeJSON(w, http.StatusOK, res)
}

func handleCreateGrade(w http.ResponseWriter, r *http.Request) {
	var g Grade
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid grade data")
		return
	}

	if g.Value < 1 || g.Value > 5 {
		writeError(w, http.StatusBadRequest, "Grade value must be between 1 and 5")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, ok := db.students[g.StudentID]; !ok {
		writeError(w, http.StatusBadRequest, "Student not found")
		return
	}
	if _, ok := db.subjects[g.SubjectID]; !ok {
		writeError(w, http.StatusBadRequest, "Subject not found")
		return
	}

	db.gradeSeq++
	g.ID = db.gradeSeq
	db.grades[g.ID] = g
	writeJSON(w, http.StatusCreated, g)
}

func handleStudentGrades(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid student ID")
		return
	}

	subjectFilterStr := r.URL.Query().Get("subject_id")
	var subjectFilter int
	if subjectFilterStr != "" {
		fmt.Sscanf(subjectFilterStr, "%d", &subjectFilter)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, ok := db.students[studentID]; !ok {
		writeError(w, http.StatusNotFound, "Student not found")
		return
	}

	var res []Grade
	for _, g := range db.grades {
		if g.StudentID == studentID {
			if subjectFilter == 0 || g.SubjectID == subjectFilter {
				res = append(res, g)
			}
		}
	}
	if res == nil {
		res = []Grade{}
	}
	writeJSON(w, http.StatusOK, res)
}

func handleGradeItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	g, exists := db.grades[id]
	if !exists {
		writeError(w, http.StatusNotFound, "Grade not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, g)
	case http.MethodDelete:
		delete(db.grades, id)
		w.WriteHeader(http.StatusOK)
	}
}


func handleHomeworks(w http.ResponseWriter, r *http.Request) {
	subjectFilterStr := r.URL.Query().Get("subject_id")
	overdueStr := r.URL.Query().Get("overdue")

	var subFilter int
	if subjectFilterStr != "" {
		fmt.Sscanf(subjectFilterStr, "%d", &subFilter)
	}
	isOverdue := overdueStr == "true"

	db.mu.Lock()
	defer db.mu.Unlock()

	now := time.Now().Format("2006-01-02")

	var res []Homework
	for _, hw := range db.homeworks {
		matchSub := (subFilter == 0 || hw.SubjectID == subFilter)
		matchOverdue := true

		if isOverdue {
			matchOverdue = hw.Deadline < now
		}

		if matchSub && matchOverdue {
			res = append(res, hw)
		}
	}
	if res == nil {
		res = []Homework{}
	}
	writeJSON(w, http.StatusOK, res)
}

func handleCreateHomework(w http.ResponseWriter, r *http.Request) {
	var hw Homework
	if err := json.NewDecoder(r.Body).Decode(&hw); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid homework data")
		return
	}

	tIssued, err1 := time.Parse("2006-01-02", hw.IssuedDate)
	tDead, err2 := time.Parse("2006-01-02", hw.Deadline)
	if err1 != nil || err2 != nil || tDead.Before(tIssued) {
		writeError(w, http.StatusBadRequest, "Invalid dates")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, ok := db.subjects[hw.SubjectID]; !ok {
		writeError(w, http.StatusBadRequest, "Subject not found")
		return
	}

	db.hwSeq++
	hw.ID = db.hwSeq
	db.homeworks[hw.ID] = hw
	writeJSON(w, http.StatusCreated, hw)
}

func handleHomeworkItem(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "Invalid ID")
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	hw, exists := db.homeworks[id]
	if !exists {
		writeError(w, http.StatusNotFound, "Homework not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, hw)
	case http.MethodDelete:
		delete(db.homeworks, id)
		w.WriteHeader(http.StatusOK)
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "index.html")
	})

	mux.HandleFunc("POST /auth/register", handleRegister)
	mux.HandleFunc("POST /auth/login", handleLogin)
	mux.HandleFunc("GET /me", authMiddleware(handleMe))

	mux.HandleFunc("GET /students", authMiddleware(handleStudents))
	mux.HandleFunc("POST /students", authMiddleware(handleCreateStudent))
	mux.HandleFunc("GET /students/{id}", authMiddleware(handleStudentItem))
	mux.HandleFunc("PUT /students/{id}", authMiddleware(handleStudentItem))
	mux.HandleFunc("PATCH /students/{id}", authMiddleware(handleStudentItem))
	mux.HandleFunc("DELETE /students/{id}", authMiddleware(handleStudentItem))

	mux.HandleFunc("GET /teachers", authMiddleware(handleTeachers))
	mux.HandleFunc("POST /teachers", authMiddleware(handleCreateTeacher))
	mux.HandleFunc("GET /teachers/{id}", authMiddleware(handleTeacherItem))
	mux.HandleFunc("PUT /teachers/{id}", authMiddleware(handleTeacherItem))
	mux.HandleFunc("DELETE /teachers/{id}", authMiddleware(handleTeacherItem))


	mux.HandleFunc("GET /subjects", authMiddleware(handleSubjects))
	mux.HandleFunc("POST /subjects", authMiddleware(handleCreateSubject))
	mux.HandleFunc("GET /subjects/{id}", authMiddleware(handleSubjectItem))
	mux.HandleFunc("DELETE /subjects/{id}", authMiddleware(handleSubjectItem))

	mux.HandleFunc("GET /grades", authMiddleware(handleGrades))
	mux.HandleFunc("POST /grades", authMiddleware(handleCreateGrade))
	mux.HandleFunc("GET /grades/{id}", authMiddleware(handleGradeItem))
	mux.HandleFunc("DELETE /grades/{id}", authMiddleware(handleGradeItem))
	mux.HandleFunc("GET /students/{id}/grades", authMiddleware(handleStudentGrades))

	mux.HandleFunc("GET /homework", authMiddleware(handleHomeworks))
	mux.HandleFunc("POST /homework", authMiddleware(handleCreateHomework))
	mux.HandleFunc("GET /homework/{id}", authMiddleware(handleHomeworkItem))
	mux.HandleFunc("DELETE /homework/{id}", authMiddleware(handleHomeworkItem))

	fmt.Println("Server is running on http://localhost:8080")
	http.ListenAndServe(":8080", mux)
}
