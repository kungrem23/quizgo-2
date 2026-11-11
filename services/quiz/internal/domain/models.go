package quiz

// import "database/sql"

type QuizVisibility string

const (
	QuizPrivate QuizVisibility = "private"
	QuizPublic  QuizVisibility = "public"
)

func (v QuizVisibility) Valid() bool { return v == QuizPrivate || v == QuizPublic }

// Empty visibility is treated as public only for legacy in-memory values. Rows
// persisted after migration always contain an explicit value.
func (v QuizVisibility) Public() bool { return v == "" || v == QuizPublic }

type Quiz struct {
	Id         int
	Title      string         `json:"title"`
	AuthorId   int            `json:"author_id"`
	Visibility QuizVisibility `json:"visibility"`
	Questions  []*Question
}

func NewQuiz() *Quiz {
	return &Quiz{}
}

type User struct {
	Id           int
	Username     string
	PasswordHash string
}

func NewUser() *User {
	return &User{}
}

type Question struct {
	Id          int
	QuizId      int    `json:"quiz_id"`
	Position    int    `json:"position"`
	TextContent string `json:"text_content"`
	ImageId     string `json:"image_id"`
	Answers     []Answer
}

func NewQuestion() *Question {
	return &Question{}
}

type Image struct {
	Id       string
	ImageURL string
}

func NewImage() *Image {
	return &Image{}
}

type Answer struct {
	Id          int
	TextContent string
	IsCorrect   bool
	QuestionId  int
}

func NewAnswer() *Answer {
	return &Answer{}
}
