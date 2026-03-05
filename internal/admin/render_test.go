package admin

import (
	"bytes"
	"testing"

	"github.com/Map130/universities/internal/models"
	surrealmodels "github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestRenderFormNewUniversity проверяет, что шаблон form.html
// рендерится без ошибок при создании нового вуза (пустая структура University).
func TestRenderFormNewUniversity(t *testing.T) {
	r := NewRenderer("../../views", true)

	data := PageData{
		Title:     "Новый вуз",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "universities",
		Content: UniversityFormData{
			IsEdit:     false,
			RecordID:   "",
			University: models.University{},
		},
	}

	// Тест полной страницы (layout + content).
	var buf bytes.Buffer
	if err := r.renderFull("universities/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for new university form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())

	// Тест фрагмента (только content, как при HTMX-запросе).
	buf.Reset()
	if err := r.renderFragment("universities/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed for new university form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderFormEditUniversity проверяет рендеринг формы редактирования
// с заполненными данными (включая pointer-поля).
func TestRenderFormEditUniversity(t *testing.T) {
	r := NewRenderer("../../views", true)

	website := "https://kaznu.kz"
	description := "Один из ведущих вузов Казахстана"
	logoURL := "http://localhost:9000/logos/test.png"

	data := PageData{
		Title:     "Редактирование вуза",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "universities",
		Content: UniversityFormData{
			IsEdit:   true,
			RecordID: "abc123",
			University: models.University{
				Name: models.LocalizedName{
					KZ: "Әл-Фараби атындағы ҚазҰУ",
					RU: "КазНУ им. аль-Фараби",
					EN: "Al-Farabi KazNU",
				},
				Abbr:        "КазНУ",
				City:        "Алматы",
				Type:        models.UniversityTypePublic,
				LogoURL:     &logoURL,
				Website:     &website,
				Description: &description,
				CustomCSS:   ".hero { color: red; }",
			},
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("universities/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for edit university form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}

// TestRenderFormNilPointerFields проверяет, что шаблон корректно
// обрабатывает nil-указатели в полях University.
func TestRenderFormNilPointerFields(t *testing.T) {
	r := NewRenderer("../../views", true)

	// Все pointer-поля — nil (LogoURL, Website, Description).
	data := PageData{
		Title:     "Новый вуз",
		Admin:     AdminData{},
		ActiveNav: "universities",
		Content: UniversityFormData{
			IsEdit:   false,
			RecordID: "",
			University: models.University{
				Name: models.LocalizedName{RU: "Тест"},
				Type: models.UniversityTypePrivate,
			},
		},
		Errors: map[string]string{
			"name_ru": "Слишком короткое название",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFragment("universities/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed with nil pointer fields: %v", err)
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderUniversitiesIndex проверяет рендеринг списка вузов.
func TestRenderUniversitiesIndex(t *testing.T) {
	r := NewRenderer("../../views", true)

	data := PageData{
		Title:     "Вузы",
		Admin:     AdminData{Email: "admin@test.com"},
		ActiveNav: "universities",
		Content: UniversitiesListData{
			Universities: nil, // пустой список
			Search:       "",
			TypeFilter:   "",
			CityFilter:   "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("universities/index.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for universities index: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}

// TestRenderSpecialtiesIndex проверяет рендеринг списка специальностей.
func TestRenderSpecialtiesIndex(t *testing.T) {
	r := NewRenderer("../../views", true)

	// Пустой список.
	data := PageData{
		Title:     "Специальности",
		Admin:     AdminData{Email: "admin@test.com"},
		ActiveNav: "specialties",
		Content: SpecialtiesListData{
			Specialties: nil,
			Groups:      nil,
			Search:      "",
			GroupFilter: "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("specialties/index.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for specialties index (empty): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull (empty) OK, output size: %d bytes", buf.Len())

	// Список с данными.
	groupID := surrealmodels.NewRecordID("specialty_group", "grp1")
	specID := surrealmodels.NewRecordID("specialty", "spec1")

	data2 := PageData{
		Title:     "Специальности",
		Admin:     AdminData{Email: "admin@test.com", Name: "Admin"},
		ActiveNav: "specialties",
		Content: SpecialtiesListData{
			Specialties: []SpecialtyListItem{
				{
					ID:        &specID,
					Code:      "6B06101",
					Name:      models.LocalizedName{KZ: "Ақпараттық жүйелер", RU: "Информационные системы", EN: "Information Systems"},
					GroupCode: "B057",
					GroupName: "Информационные технологии",
				},
			},
			Groups: []models.SpecialtyGroup{
				{
					ID:   &groupID,
					Code: "B057",
					Name: models.LocalizedName{KZ: "Ақпараттық технологиялар", RU: "Информационные технологии", EN: "Information Technologies"},
				},
			},
			Search:      "информ",
			GroupFilter: "B057",
		},
	}

	buf.Reset()
	if err := r.renderFull("specialties/index.html", data2, &buf); err != nil {
		t.Fatalf("renderFull failed for specialties index (with data): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull (with data) OK, output size: %d bytes", buf.Len())
}

// TestRenderSpecialtyFormNew проверяет рендеринг формы создания специальности.
func TestRenderSpecialtyFormNew(t *testing.T) {
	r := NewRenderer("../../views", true)

	groupID := surrealmodels.NewRecordID("specialty_group", "grp1")

	data := PageData{
		Title:     "Новая специальность",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "specialties",
		Content: SpecialtyFormData{
			IsEdit:    false,
			RecordID:  "",
			Specialty: models.Specialty{},
			Groups: []models.SpecialtyGroup{
				{
					ID:   &groupID,
					Code: "B057",
					Name: models.LocalizedName{KZ: "Ақпараттық технологиялар", RU: "Информационные технологии", EN: "IT"},
				},
			},
			SelectedGroupID: "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("specialties/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for new specialty form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())

	// HTMX fragment.
	buf.Reset()
	if err := r.renderFragment("specialties/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed for new specialty form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderSpecialtyFormEdit проверяет рендеринг формы редактирования специальности.
func TestRenderSpecialtyFormEdit(t *testing.T) {
	r := NewRenderer("../../views", true)

	groupID := surrealmodels.NewRecordID("specialty_group", "grp1")
	specID := surrealmodels.NewRecordID("specialty", "spec1")

	data := PageData{
		Title:     "Редактирование — 6B06101",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "specialties",
		Content: SpecialtyFormData{
			IsEdit:   true,
			RecordID: "spec1",
			Specialty: models.Specialty{
				ID:   &specID,
				Code: "6B06101",
				Name: models.LocalizedName{
					KZ: "Ақпараттық жүйелер",
					RU: "Информационные системы",
					EN: "Information Systems",
				},
				Group: surrealmodels.NewRecordID("specialty_group", "grp1"),
			},
			Groups: []models.SpecialtyGroup{
				{
					ID:   &groupID,
					Code: "B057",
					Name: models.LocalizedName{KZ: "Ақпараттық технологиялар", RU: "Информационные технологии", EN: "IT"},
				},
			},
			SelectedGroupID: "grp1",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("specialties/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for edit specialty form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}

// TestRenderSpecialtyFormWithErrors проверяет рендеринг формы специальности с ошибками валидации.
func TestRenderSpecialtyFormWithErrors(t *testing.T) {
	r := NewRenderer("../../views", true)

	data := PageData{
		Title:     "Новая специальность",
		Admin:     AdminData{},
		ActiveNav: "specialties",
		Content: SpecialtyFormData{
			IsEdit:   false,
			RecordID: "",
			Specialty: models.Specialty{
				Code: "",
				Name: models.LocalizedName{RU: "Тест"},
			},
			Groups:          []models.SpecialtyGroup{},
			SelectedGroupID: "",
		},
		Errors: map[string]string{
			"code":     "Код специальности обязателен",
			"name_kz":  "Название на казахском обязательно",
			"name_en":  "Название на английском обязательно",
			"group_id": "Выберите группу образовательных программ",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFragment("specialties/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed with validation errors: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderGroupsIndex проверяет рендеринг списка групп ОП.
func TestRenderGroupsIndex(t *testing.T) {
	r := NewRenderer("../../views", true)

	// Пустой список.
	data := PageData{
		Title:     "Группы ОП",
		Admin:     AdminData{Email: "admin@test.com"},
		ActiveNav: "groups",
		Content: GroupsListData{
			Groups: nil,
			Search: "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("groups/index.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for groups index (empty): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull (empty) OK, output size: %d bytes", buf.Len())

	// Список с данными.
	groupID := surrealmodels.NewRecordID("specialty_group", "grp1")

	data2 := PageData{
		Title:     "Группы ОП",
		Admin:     AdminData{Email: "admin@test.com", Name: "Admin"},
		ActiveNav: "groups",
		Content: GroupsListData{
			Groups: []models.SpecialtyGroup{
				{
					ID:   &groupID,
					Code: "B057",
					Name: models.LocalizedName{KZ: "Ақпараттық технологиялар", RU: "Информационные технологии", EN: "Information Technologies"},
				},
			},
			Search: "информ",
		},
	}

	buf.Reset()
	if err := r.renderFull("groups/index.html", data2, &buf); err != nil {
		t.Fatalf("renderFull failed for groups index (with data): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull (with data) OK, output size: %d bytes", buf.Len())
}

// TestRenderGroupFormNew проверяет рендеринг формы создания группы ОП.
func TestRenderGroupFormNew(t *testing.T) {
	r := NewRenderer("../../views", true)

	subjectID := surrealmodels.NewRecordID("subject", "math1")

	data := PageData{
		Title:     "Новая группа ОП",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:   false,
			RecordID: "",
			Group:    models.SpecialtyGroup{},
			AllSubjects: []models.Subject{
				{
					ID:   &subjectID,
					Name: models.LocalizedName{KZ: "Математика", RU: "Математика", EN: "Mathematics"},
				},
			},
			RequiredSubjects: []models.RequiredSubject{},
			SelectedSubject1: "",
			SelectedSubject2: "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("groups/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for new group form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())

	// HTMX fragment.
	buf.Reset()
	if err := r.renderFragment("groups/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed for new group form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderGroupFormEdit проверяет рендеринг формы редактирования группы ОП
// с привязанными предметами ЕНТ (requires edges).
func TestRenderGroupFormEdit(t *testing.T) {
	r := NewRenderer("../../views", true)

	groupID := surrealmodels.NewRecordID("specialty_group", "grp1")
	subjectID1 := surrealmodels.NewRecordID("subject", "math1")
	subjectID2 := surrealmodels.NewRecordID("subject", "phys1")
	requiresID1 := surrealmodels.NewRecordID("requires", "req1")
	requiresID2 := surrealmodels.NewRecordID("requires", "req2")

	data := PageData{
		Title:     "Редактирование — B057",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:   true,
			RecordID: "grp1",
			Group: models.SpecialtyGroup{
				ID:   &groupID,
				Code: "B057",
				Name: models.LocalizedName{
					KZ: "Ақпараттық технологиялар",
					RU: "Информационные технологии",
					EN: "Information Technologies",
				},
			},
			AllSubjects: []models.Subject{
				{
					ID:   &subjectID1,
					Name: models.LocalizedName{KZ: "Математика", RU: "Математика", EN: "Mathematics"},
				},
				{
					ID:   &subjectID2,
					Name: models.LocalizedName{KZ: "Физика", RU: "Физика", EN: "Physics"},
				},
			},
			RequiredSubjects: []models.RequiredSubject{
				{
					ID:       &requiresID1,
					In:       groupID,
					Out:      models.Subject{ID: &subjectID1, Name: models.LocalizedName{KZ: "Математика", RU: "Математика", EN: "Mathematics"}},
					Priority: models.SubjectPriorityProfile,
				},
				{
					ID:       &requiresID2,
					In:       groupID,
					Out:      models.Subject{ID: &subjectID2, Name: models.LocalizedName{KZ: "Физика", RU: "Физика", EN: "Physics"}},
					Priority: models.SubjectPrioritySecondary,
				},
			},
			SelectedSubject1: "math1",
			SelectedSubject2: "phys1",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("groups/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for edit group form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}

// TestRenderGroupFormEditEmpty проверяет рендеринг формы редактирования группы ОП
// без привязанных предметов ЕНТ (пустой requires list).
func TestRenderGroupFormEditEmpty(t *testing.T) {
	r := NewRenderer("../../views", true)

	groupID := surrealmodels.NewRecordID("specialty_group", "grp2")

	data := PageData{
		Title:     "Редактирование — B058",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:   true,
			RecordID: "grp2",
			Group: models.SpecialtyGroup{
				ID:   &groupID,
				Code: "B058",
				Name: models.LocalizedName{
					KZ: "Энергетика",
					RU: "Энергетика и электротехника",
					EN: "Energy and Electrical Engineering",
				},
			},
			AllSubjects:      []models.Subject{},
			RequiredSubjects: []models.RequiredSubject{},
			SelectedSubject1: "",
			SelectedSubject2: "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("groups/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for edit group form (empty requires): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}

// TestRenderGroupFormWithErrors проверяет рендеринг формы группы ОП с ошибками валидации.
func TestRenderGroupFormWithErrors(t *testing.T) {
	r := NewRenderer("../../views", true)

	data := PageData{
		Title:     "Новая группа ОП",
		Admin:     AdminData{},
		ActiveNav: "groups",
		Content: GroupFormData{
			IsEdit:   false,
			RecordID: "",
			Group: models.SpecialtyGroup{
				Code: "",
				Name: models.LocalizedName{RU: "Тест"},
			},
			AllSubjects:      []models.Subject{},
			RequiredSubjects: []models.RequiredSubject{},
			SelectedSubject1: "",
			SelectedSubject2: "",
		},
		Errors: map[string]string{
			"code":    "Код группы ОП обязателен",
			"name_kz": "Название на казахском обязательно",
			"name_en": "Название на английском обязательно",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFragment("groups/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed with validation errors: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// ────────────────────────────────────────────────────────────────────────────
//  Subjects — рендер-тесты шаблонов предметов ЕНТ
// ────────────────────────────────────────────────────────────────────────────

// TestRenderSubjectsIndex проверяет рендеринг списка предметов ЕНТ.
func TestRenderSubjectsIndex(t *testing.T) {
	r := NewRenderer("../../views", true)

	// Пустой список.
	data := PageData{
		Title:     "Предметы ЕНТ",
		Admin:     AdminData{Email: "admin@test.com"},
		ActiveNav: "subjects",
		Content: SubjectsListData{
			Subjects: nil,
			Search:   "",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("subjects/index.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for subjects index (empty): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull (empty) OK, output size: %d bytes", buf.Len())

	// Список с данными.
	subjectID1 := surrealmodels.NewRecordID("subject", "math1")
	subjectID2 := surrealmodels.NewRecordID("subject", "phys1")

	data2 := PageData{
		Title:     "Предметы ЕНТ",
		Admin:     AdminData{Email: "admin@test.com", Name: "Admin"},
		ActiveNav: "subjects",
		Content: SubjectsListData{
			Subjects: []models.Subject{
				{
					ID:   &subjectID1,
					Name: models.LocalizedName{KZ: "Математика", RU: "Математика", EN: "Mathematics"},
				},
				{
					ID:   &subjectID2,
					Name: models.LocalizedName{KZ: "Физика", RU: "Физика", EN: "Physics"},
				},
			},
			Search: "матем",
		},
	}

	buf.Reset()
	if err := r.renderFull("subjects/index.html", data2, &buf); err != nil {
		t.Fatalf("renderFull failed for subjects index (with data): %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull (with data) OK, output size: %d bytes", buf.Len())
}

// TestRenderSubjectFormNew проверяет рендеринг формы создания предмета ЕНТ.
func TestRenderSubjectFormNew(t *testing.T) {
	r := NewRenderer("../../views", true)

	data := PageData{
		Title:     "Новый предмет ЕНТ",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "subjects",
		Content: SubjectFormData{
			IsEdit:   false,
			RecordID: "",
			Subject:  models.Subject{},
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("subjects/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for new subject form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())

	// HTMX fragment.
	buf.Reset()
	if err := r.renderFragment("subjects/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed for new subject form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderSubjectFormEdit проверяет рендеринг формы редактирования предмета ЕНТ.
func TestRenderSubjectFormEdit(t *testing.T) {
	r := NewRenderer("../../views", true)

	subjectID := surrealmodels.NewRecordID("subject", "math1")

	data := PageData{
		Title:     "Редактирование — Математика",
		Admin:     AdminData{Email: "test@test.com", Name: "Test"},
		ActiveNav: "subjects",
		Content: SubjectFormData{
			IsEdit:   true,
			RecordID: "math1",
			Subject: models.Subject{
				ID: &subjectID,
				Name: models.LocalizedName{
					KZ: "Математика",
					RU: "Математика",
					EN: "Mathematics",
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("subjects/form.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for edit subject form: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}

// TestRenderSubjectFormWithErrors проверяет рендеринг формы предмета ЕНТ с ошибками валидации.
func TestRenderSubjectFormWithErrors(t *testing.T) {
	r := NewRenderer("../../views", true)

	data := PageData{
		Title:     "Новый предмет ЕНТ",
		Admin:     AdminData{},
		ActiveNav: "subjects",
		Content: SubjectFormData{
			IsEdit:   false,
			RecordID: "",
			Subject: models.Subject{
				Name: models.LocalizedName{RU: ""},
			},
		},
		Errors: map[string]string{
			"name_ru": "Название на русском обязательно",
			"name_kz": "Название на казахском обязательно",
			"name_en": "Название на английском обязательно",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFragment("subjects/form.html", data, &buf); err != nil {
		t.Fatalf("renderFragment failed with validation errors: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFragment returned empty output")
	}
	t.Logf("renderFragment OK, output size: %d bytes", buf.Len())
}

// TestRenderStub проверяет рендеринг stub-страниц.
func TestRenderStub(t *testing.T) {
	r := NewRenderer("../../views", true)

	type stubContent struct {
		Title       string
		Description string
	}

	data := PageData{
		Title:     "Специальности",
		Admin:     AdminData{Email: "admin@test.com"},
		ActiveNav: "specialties",
		Content: stubContent{
			Title:       "Специальности",
			Description: "В разработке",
		},
	}

	var buf bytes.Buffer
	if err := r.renderFull("stub.html", data, &buf); err != nil {
		t.Fatalf("renderFull failed for stub page: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("renderFull returned empty output")
	}
	t.Logf("renderFull OK, output size: %d bytes", buf.Len())
}
