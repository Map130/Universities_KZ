package admin

import (
	"bytes"
	"testing"

	"github.com/Map130/universities/internal/models"
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
