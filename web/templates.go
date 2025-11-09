package web

import (
	"bytes"
	"embed"
	"html/template"
	"io"
	"time"

	"github.com/davidkleiven/caesura/pkg"
	"github.com/davidkleiven/caesura/utils"
)

//go:embed templates/*
var templatesFS embed.FS

type ScoreMetaData struct {
	Composer string
	Arranger string
	Title    string
}

func translateFunc(language string) func(string) string {
	return func(f string) string { return translator.MustGet(language, f) }
}

func Upload(data *ScoreMetaData, language string) []byte {
	tmpl := template.Must(
		template.New("upload").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/upload.html", "templates/header.html", "templates/footer.html"),
	)
	var buf bytes.Buffer

	deps := LoadDependencies().Dependencies
	templateData := struct {
		ScoreMetaData *ScoreMetaData
		Dependencies  *Dependencies
	}{
		ScoreMetaData: data,
		Dependencies:  &deps,
	}

	pkg.PanicOnErr(tmpl.ExecuteTemplate(&buf, "upload", templateData))
	return buf.Bytes()
}

func List() []byte {
	return utils.Must(templatesFS.ReadFile("templates/list.html"))
}

func Index(language string) []byte {
	tmpl := template.Must(
		template.New("index-template").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/index.html", "templates/header.html"),
	)

	var buf bytes.Buffer

	pkg.PanicOnErr(tmpl.ExecuteTemplate(&buf, "index-template", LoadDependencies().Dependencies))
	return buf.Bytes()
}

func Overview(language string) []byte {
	tmpl := template.Must(
		template.New("overview").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/overview.html", "templates/header.html", "templates/resource_table.html", "templates/footer.html"),
	)
	var buf bytes.Buffer
	pkg.PanicOnErr(tmpl.ExecuteTemplate(&buf, "overview", LoadDependencies().Dependencies))
	return buf.Bytes()
}

func ResourceList(w io.Writer, metaData []pkg.MetaData) {
	data := ResourceListData{
		MetaData:                 metaData,
		CheckboxVisible:          true,
		PatchVisible:             true,
		RemoveFromProjectVisible: false,
	}
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/resource_list.html"))
	pkg.PanicOnErr(tmpl.Execute(w, data))
}

func ProjectSelectorModal(language string) []byte {
	tmpl := template.Must(
		template.New("project-modal").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/project_selection_modal.html"),
	)
	var buf bytes.Buffer
	pkg.PanicOnErr(tmpl.ExecuteTemplate(&buf, "project-modal", LoadDependencies().Dependencies))
	return buf.Bytes()
}

func ProjectQueryInput(w io.Writer, language, queryContent string) {
	tmpl := template.Must(
		template.New("project-query-input").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/project_query_input.html"),
	)
	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "project-query-input", queryContent))
}

func Projects(language string) []byte {
	tmpl := template.Must(
		template.New("projects").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/projects.html", "templates/header.html", "templates/footer.html"),
	)
	var buf bytes.Buffer
	pkg.PanicOnErr(tmpl.ExecuteTemplate(&buf, "projects", LoadDependencies().Dependencies))
	return buf.Bytes()
}

func ProjectList(w io.Writer, projects []pkg.Project) {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/project_list.html"))

	data := make([]struct {
		Name      string
		Id        string
		CreatedAt string
		UpdatedAt string
		NumPieces int
	}, len(projects))
	for i, project := range projects {
		data[i].Name = project.Name
		data[i].Id = project.Id()
		data[i].CreatedAt = project.CreatedAt.Format(time.RFC1123)
		data[i].UpdatedAt = project.UpdatedAt.Format(time.RFC1123)
		data[i].NumPieces = len(project.ResourceIds)
	}

	pkg.PanicOnErr(tmpl.Execute(w, data))
}

func ProjectContent(w io.Writer, project *pkg.Project, resources []pkg.MetaData, language string) {
	resourceTable := template.Must(
		template.New("project-content").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/project_content.html", "templates/resource_table.html"),
	)

	var resourceTableBuffer bytes.Buffer
	pkg.PanicOnErr(resourceTable.ExecuteTemplate(&resourceTableBuffer, "project-content", project))

	var buffer bytes.Buffer
	rows := template.Must(template.ParseFS(templatesFS, "templates/resource_list.html"))

	data := ResourceListData{
		MetaData:                 resources,
		CheckboxVisible:          false,
		PatchVisible:             false,
		RemoveFromProjectVisible: true,
		ProjectId:                project.Id(),
	}

	pkg.PanicOnErr(rows.Execute(&buffer, data))

	buffer.Write([]byte("</tbody>"))
	w.Write(bytes.ReplaceAll(resourceTableBuffer.Bytes(), []byte("</tbody>"), buffer.Bytes()))
}

type ResourceListData struct {
	MetaData                 []pkg.MetaData
	ProjectId                string
	CheckboxVisible          bool
	PatchVisible             bool
	RemoveFromProjectVisible bool
}

type ResourceContentData struct {
	ResourceId string
	Filenames  []string
}

func ResourceContent(w io.Writer, data *ResourceContentData) {
	template := template.Must(template.ParseFS(templatesFS, "templates/resource_content.html"))
	pkg.PanicOnErr(template.Execute(w, data))
}

func Organizations(language string) []byte {
	tmpl := template.Must(
		template.New("organizations").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/organizations.html", "templates/header.html", "templates/organization_list.html", "templates/footer.html"),
	)
	var buf bytes.Buffer

	pkg.PanicOnErr(tmpl.ExecuteTemplate(&buf, "organizations", LoadDependencies()))
	return buf.Bytes()
}

func WriteOrganizationHTML(w io.Writer, organizations []pkg.Organization) {
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/organization_list.html"))
	data := struct {
		Organizations []pkg.Organization
	}{
		Organizations: organizations,
	}
	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "orgList", data))
}

func WritePeopleHTML(w io.Writer, language string) {
	tmpl := template.Must(
		template.New("people").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/people.html", "templates/header.html", "templates/footer.html"),
	)
	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "people", LoadDependencies()))
}

type userListViewObj struct {
	Id        string
	Name      string
	Email     string
	Roles     []pkg.RoleKind
	Groups    []string
	GroupOpts []Option
}

func WriteUserList(w io.Writer, users []pkg.UserInfo, orgId string, groupOpts []string) {
	tmpl := template.Must(
		template.New("userList").Funcs(template.FuncMap{
			"getRoleName": getRoleName,
		}).ParseFS(templatesFS, "templates/user_list.html", "templates/options.html"),
	)
	viewObj := make([]userListViewObj, len(users))

	roleOpts := map[pkg.RoleKind][]pkg.RoleKind{
		pkg.RoleViewer: {pkg.RoleViewer, pkg.RoleEditor, pkg.RoleAdmin},
		pkg.RoleEditor: {pkg.RoleEditor, pkg.RoleAdmin, pkg.RoleViewer},
		pkg.RoleAdmin:  {pkg.RoleAdmin, pkg.RoleViewer, pkg.RoleEditor},
	}

	opts := make([]Option, len(groupOpts))
	for i, g := range groupOpts {
		opts[i].Value = g
		opts[i].Name = g
	}

	for i, u := range users {
		viewObj[i] = userListViewObj{
			Id:        u.Id,
			Email:     u.Email,
			Name:      u.Name,
			Roles:     roleOpts[u.Roles[orgId]],
			Groups:    u.Groups[orgId],
			GroupOpts: opts,
		}
	}

	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "userList", viewObj))
}

func getRoleName(r pkg.RoleKind) string {
	switch r {
	case pkg.RoleViewer:
		return "Viewer"
	case pkg.RoleEditor:
		return "Editor"
	case pkg.RoleAdmin:
		return "Admin"
	default:
		return ""
	}
}

type Option struct {
	Value string
	Name  string
}

func WriteStringAsOptions(w io.Writer, items []string) {
	options := make([]Option, len(items))
	for i, item := range items {
		options[i].Name = item
		options[i].Value = item
	}
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/options.html"))
	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "option-list", options))
}

func SignIn(lang string) string {
	signIn := translator.MustGet(lang, "sign-in")
	return `<a href="/login">` + signIn + "</a>"
}

func SignedIn(lang string) string {
	return translator.MustGet(lang, "signed-in")
}

func NoOrganization(lang string) string {
	return translator.MustGet(lang, "no-org")
}

func SubscriptionExpired(lang string) string {
	return translator.MustGet(lang, "org.subscription-expired")
}

func SubscriptionExpires(lang string) string {
	return translator.MustGet(lang, "org.subscription-expires")
}

func MaxNumScoresReached(lang string) string {
	return translator.MustGet(lang, "org.max-num-scores-reached")
}

func LoginForm(w io.Writer, language string) {
	tmpl := template.Must(
		template.New("login").
			Funcs(template.FuncMap{"T": translateFunc(language)}).
			ParseFS(templatesFS, "templates/login.html", "templates/header.html", "templates/footer.html"),
	)

	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "login", LoadDependencies()))
}

func MinimumPasswordLength(lang string) string {
	return translator.MustGet(lang, "login.minimum_password_length")
}

func UserNotFound(w io.Writer, lang string, email string) {
	templText := translator.MustGet(lang, "login.user_not_found")
	templ := utils.Must(template.New("msg").Parse(templText))

	data := struct {
		Email string
	}{
		Email: email,
	}
	pkg.PanicOnErr(templ.Execute(w, data))
}

func SuccessfulLogin(lang string) string {
	return translator.MustGet(lang, "login.success")
}

func UserAlreadyExist(w io.Writer, lang, email string) {
	templText := translator.MustGet(lang, "login.user_exists")
	templ := utils.Must(template.New("msg").Parse(templText))

	data := struct {
		Email string
	}{
		Email: email,
	}
	pkg.PanicOnErr(templ.Execute(w, data))

}

func PasswordAndRetypedPasswordMustMatch(lang string) string {
	return translator.MustGet(lang, "login.password_must_match")
}

func Unauthorized(lang string) string {
	return translator.MustGet(lang, "login.unauthorized")
}

func EnterValidEmail(w io.Writer, lang string) {
	w.Write([]byte(translator.MustGet(lang, "login.enter_valid_email")))
}

func ResetEmailSent(w io.Writer, lang string, email string) {
	templText := translator.MustGet(lang, "login.reset_email_sent")
	templ := utils.Must(template.New("msg").Parse(templText))

	data := struct {
		Email string
	}{
		Email: email,
	}
	pkg.PanicOnErr(templ.Execute(w, data))
}

func ResetPasswordPage(w io.Writer, lang string) {
	tmpl := template.Must(
		template.New("resetPassword").
			Funcs(template.FuncMap{"T": translateFunc(lang)}).
			ParseFS(templatesFS, "templates/resetPassword.html", "templates/header.html", "templates/footer.html"),
	)
	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "resetPassword", LoadDependencies()))

}

func CreateNewProject(lang string) string {
	return translator.MustGet(lang, "project-modal.create-new")
}

func AboutUsPage(w io.Writer, lang string) {
	tmpl := template.Must(
		template.New("contact").
			Funcs(template.FuncMap{"T": translateFunc(lang)}).
			ParseFS(templatesFS, "templates/about.html", "templates/header.html", "templates/footer.html"),
	)
	pkg.PanicOnErr(tmpl.ExecuteTemplate(w, "contact", nil))
}
