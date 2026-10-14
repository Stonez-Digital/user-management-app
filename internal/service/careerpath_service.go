package service

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

const CareerPathAssessmentVersion = "careerpath-v2"

var (
	ErrCareerPathForbidden = errors.New("careerpath access forbidden")
	ErrCareerPathNotFound = errors.New("careerpath attempt not found")
	ErrCareerPathSubmitted = errors.New("careerpath attempt already submitted")
	ErrCareerPathInvalidResponse = errors.New("careerpath response invalid")
	ErrCareerPathIncomplete = errors.New("careerpath assessment incomplete")
	ErrCareerPathVersionMismatch = errors.New("careerpath assessment version mismatch")
	ErrCareerPathConflict = errors.New("careerpath attempt changed; reload and retry")
)

type CareerPathOption struct { ID string `json:"id"`; Label string `json:"label"`; Tags []string `json:"-"` }
type CareerPathQuestion struct { ID string `json:"id"`; Prompt string `json:"prompt"`; Help string `json:"help,omitempty"`; Options []CareerPathOption `json:"options"`; Multiple bool `json:"multiple"` }
type CareerPathAssessment struct { ID string `json:"id"`; Version string `json:"version"`; Title string `json:"title"`; Description string `json:"description"`; Questions []CareerPathQuestion `json:"questions"` }
type CareerPathCareer struct { ID string `json:"id"`; Title string `json:"title"`; Description string `json:"description"`; Subjects []string `json:"relevant_subjects"`; Skills []string `json:"skills_to_explore"`; Activity string `json:"exploration_activity"`; Tags []string `json:"-"` }
type CareerPathMatch struct { Career CareerPathCareer `json:"career"`; MatchedInterests []string `json:"matched_interests"`; SharedTags int `json:"shared_tags"`; Explanation string `json:"explanation"` }
type CareerPathAttemptView struct { ID uuid.UUID `json:"id"`; Version string `json:"version"`; Status string `json:"status"`; Responses map[string][]string `json:"responses"`; Results []CareerPathMatch `json:"results,omitempty"`; SubmittedAt *time.Time `json:"submitted_at,omitempty"`; UpdatedAt time.Time `json:"updated_at"` }

type CareerPathService struct { db *gorm.DB }
func NewCareerPathService(db *gorm.DB) *CareerPathService { return &CareerPathService{db: db} }

func careerPathAssessment() CareerPathAssessment {
	return CareerPathAssessment{
		ID: "explore-v2", Version: CareerPathAssessmentVersion,
		Title: "Explore your interests and strengths",
		Description: "Explore career ideas through your interests, strengths, preferred work settings, subjects and experiences. There are no right or wrong answers. Your results are suggestions, not a decision about your future.",
		Questions: []CareerPathQuestion{
			{ID:"interests", Prompt:"Which activities would you enjoy exploring?", Help:"Choose all that sound interesting, even if you have not tried them yet.", Multiple:true, Options:[]CareerPathOption{
				{"build","Building or fixing things",[]string{"hands_on","design","engineering"}},
				{"investigate","Investigating questions and patterns",[]string{"research","science","analysis"}},
				{"help","Helping or teaching people",[]string{"people","care","teaching"}},
				{"create","Creating stories, art or digital media",[]string{"creative","communication","design"}},
				{"organize","Planning events, projects or resources",[]string{"organize","business","leadership"}},
				{"nature","Working with plants, animals or the environment",[]string{"nature","science","hands_on"}},
			}},
			{ID:"personality_strengths", Prompt:"Which ways of working feel most natural or enjoyable to you?", Help:"This is a reflection, not a personality test or diagnosis. Select more than one if they fit.", Multiple:true, Options:[]CareerPathOption{
				{"curious","Asking questions and finding out how things work",[]string{"research","science","analysis"}},
				{"creative_thinker","Imagining new ideas or different solutions",[]string{"creative","design","engineering"}},
				{"supportive","Listening, encouraging or helping others",[]string{"people","care","teaching","communication"}},
				{"planner","Organising tasks and keeping things on track",[]string{"organize","business","leadership"}},
				{"practical","Learning by doing, building or testing",[]string{"hands_on","engineering","design"}},
				{"independent_thinker","Focusing deeply and solving problems independently",[]string{"analysis","independent","technology"}},
			}},
			{ID:"work_style", Prompt:"Which work settings sound appealing to you?", Help:"Think about the kind of day-to-day environment you might enjoy.", Multiple:true, Options:[]CareerPathOption{
				{"team","Working with a team",[]string{"people","collaboration"}},
				{"independent","Having time to focus independently",[]string{"analysis","independent"}},
				{"field","Being active outdoors or in the field",[]string{"nature","hands_on"}},
				{"studio","Making or designing things",[]string{"creative","design"}},
				{"community","Serving a school or community",[]string{"people","care","teaching"}},
			}},
			{ID:"workplace_preferences", Prompt:"What kind of workplace or tasks would you like to try?", Help:"Choose the options that appeal to you; there is no preferred or better answer.", Multiple:true, Options:[]CareerPathOption{
				{"structured","Clear steps, routines and organised tasks",[]string{"organize","business","analysis"}},
				{"flexible","Varied tasks and room to try new approaches",[]string{"creative","design","research"}},
				{"people_facing","Talking with, teaching or supporting people",[]string{"people","teaching","communication","care"}},
				{"behind_scenes","Working behind the scenes to improve a system",[]string{"technology","analysis","engineering"}},
				{"outdoors","Working outdoors or visiting different places",[]string{"nature","hands_on","science"}},
				{"digital","Using computers or digital tools to make things",[]string{"technology","design","analysis"}},
			}},
			{ID:"subjects", Prompt:"Which subjects or skills would you like to use more?", Help:"You can choose subjects you enjoy now or want to learn.", Multiple:true, Options:[]CareerPathOption{
				{"maths","Mathematics and logical problem-solving",[]string{"analysis","engineering","technology"}},
				{"science","Science and experiments",[]string{"science","research"}},
				{"language","Languages, reading or writing",[]string{"communication","creative","teaching"}},
				{"computing","Computers and digital tools",[]string{"technology","analysis","design"}},
				{"business","Business, money or entrepreneurship",[]string{"business","organize","analysis"}},
				{"arts","Art, design or practical crafts",[]string{"creative","design","hands_on"}},
			}},
			{ID:"education_experience", Prompt:"What have you already tried or would like to practise?", Help:"Choose skills or activities you have tried, or would like to try next. You do not need formal work experience.", Multiple:true, Options:[]CareerPathOption{
				{"school_project","A school project, experiment or research task",[]string{"research","science","analysis"}},
				{"digital_project","Making something with a computer or phone",[]string{"technology","design","creative"}},
				{"helping_activity","Helping, tutoring or volunteering",[]string{"people","care","teaching"}},
				{"creative_project","Writing, drawing, designing or making media",[]string{"creative","communication","design"}},
				{"practical_task","Repairing, building, growing or fixing something",[]string{"hands_on","engineering","nature"}},
				{"organising_activity","Planning an event, club activity or small project",[]string{"organize","business","leadership"}},
				{"not_yet","I have not tried these yet, but I am ready to explore",[]string{"research","learning","curiosity"}},
			}},
			{ID:"goals", Prompt:"What would you most like to explore next?", Help:"Pick the next steps that sound useful to you.", Multiple:true, Options:[]CareerPathOption{
				{"solve","Solving a real-world problem",[]string{"research","analysis","engineering"}},
				{"make","Making a useful product or service",[]string{"hands_on","design","technology"}},
				{"support","Supporting people to learn or thrive",[]string{"people","care","teaching"}},
				{"express","Sharing ideas through creative work",[]string{"creative","communication"}},
				{"improve","Improving how a group or organisation works",[]string{"business","organize","leadership"}},
				{"protect","Caring for nature or public wellbeing",[]string{"nature","science","care"}},
			}},
		},
	}
}
func careerPathCatalogue() []CareerPathCareer { return []CareerPathCareer{
	{ID:"software-developer",Title:"Software Developer",Description:"Designs and builds software that helps people solve problems.",Subjects:[]string{"Mathematics","Computing","English"},Skills:[]string{"Programming","Problem-solving","Testing"},Activity:"Build a simple study timetable or quiz app.",Tags:[]string{"technology","analysis","engineering","design","independent"}},
	{ID:"engineer",Title:"Engineer",Description:"Uses science and design to create, test or improve systems and products.",Subjects:[]string{"Mathematics","Physics","Technical drawing"},Skills:[]string{"Design","Measurement","Teamwork"},Activity:"Sketch and test a simple bridge or water-saving device.",Tags:[]string{"engineering","hands_on","analysis","science","design"}},
	{ID:"health-professional",Title:"Health Professional",Description:"Supports health and wellbeing through care, education or clinical services.",Subjects:[]string{"Biology","Chemistry","English"},Skills:[]string{"Communication","Care","Careful observation"},Activity:"Create a fact-checked community health information poster.",Tags:[]string{"care","people","science","communication"}},
	{ID:"teacher",Title:"Teacher or Learning Facilitator",Description:"Helps learners understand ideas and develop useful skills.",Subjects:[]string{"Relevant subject","English","Education"},Skills:[]string{"Explaining ideas","Listening","Planning"},Activity:"Prepare and deliver a five-minute lesson on a topic you enjoy.",Tags:[]string{"teaching","people","communication","collaboration","organize"}},
	{ID:"designer",Title:"Digital or Product Designer",Description:"Plans how a product looks, feels and works for the people who use it.",Subjects:[]string{"Art and design","Computing","English"},Skills:[]string{"Visual communication","Research","Prototyping"},Activity:"Redesign a school notice or sketch a mobile app screen.",Tags:[]string{"creative","design","technology","communication","hands_on"}},
	{ID:"environmental-scientist",Title:"Environmental Scientist",Description:"Studies environmental questions and helps communities make informed decisions.",Subjects:[]string{"Biology","Geography","Chemistry"},Skills:[]string{"Observation","Research","Data interpretation"},Activity:"Record local waste or plant observations and suggest one improvement.",Tags:[]string{"nature","science","research","analysis","hands_on"}},
	{ID:"business-operations",Title:"Business and Operations",Description:"Helps organisations plan services, coordinate work and use resources responsibly.",Subjects:[]string{"Business studies","Mathematics","English"},Skills:[]string{"Planning","Communication","Budgeting"},Activity:"Plan a small school event with a timeline and sample budget.",Tags:[]string{"business","organize","leadership","analysis","people"}},
	{ID:"journalism",Title:"Journalism and Communication",Description:"Researches and explains information for audiences through writing or media.",Subjects:[]string{"English","Civic education","Media studies"},Skills:[]string{"Research","Writing","Interviewing"},Activity:"Write a short, balanced report about a school or community project.",Tags:[]string{"creative","communication","research","people"}},
} }

func careerPathAssessmentV1() CareerPathAssessment {
	return CareerPathAssessment{ID:"explore-v1", Version:CareerPathAssessmentVersion, Title:"Explore your interests", Description:"A short reflection to help you explore possible careers. There are no right or wrong answers; your results are suggestions, not a decision about your future.", Questions:[]CareerPathQuestion{
		{ID:"interests", Prompt:"Which activities would you enjoy exploring?", Multiple:true, Options:[]CareerPathOption{{"build","Building or fixing things",[]string{"hands_on","design","engineering"}},{"investigate","Investigating questions and patterns",[]string{"research","science","analysis"}},{"help","Helping or teaching people",[]string{"people","care","teaching"}},{"create","Creating stories, art or digital media",[]string{"creative","communication","design"}},{"organize","Planning events, projects or resources",[]string{"organize","business","leadership"}},{"nature","Working with plants, animals or the environment",[]string{"nature","science","hands_on"}}}},
		{ID:"work_style", Prompt:"Which work settings sound appealing to you?", Multiple:true, Options:[]CareerPathOption{{"team","Working with a team",[]string{"people","collaboration"}},{"independent","Having time to focus independently",[]string{"analysis","independent"}},{"field","Being active outdoors or in the field",[]string{"nature","hands_on"}},{"studio","Making or designing things",[]string{"creative","design"}},{"community","Serving a school or community",[]string{"people","care","teaching"}}}},
		{ID:"subjects", Prompt:"Which subjects or skills would you like to use more?", Multiple:true, Options:[]CareerPathOption{{"maths","Mathematics and logical problem-solving",[]string{"analysis","engineering","technology"}},{"science","Science and experiments",[]string{"science","research"}},{"language","Languages, reading or writing",[]string{"communication","creative","teaching"}},{"computing","Computers and digital tools",[]string{"technology","analysis","design"}},{"business","Business, money or entrepreneurship",[]string{"business","organize","analysis"}},{"arts","Art, design or practical crafts",[]string{"creative","design","hands_on"}}}},
		{ID:"goals", Prompt:"What would you most like to explore next?", Multiple:true, Options:[]CareerPathOption{{"solve","Solving a real-world problem",[]string{"research","analysis","engineering"}},{"make","Making a useful product or service",[]string{"hands_on","design","technology"}},{"support","Supporting people to learn or thrive",[]string{"people","care","teaching"}},{"express","Sharing ideas through creative work",[]string{"creative","communication"}},{"improve","Improving how a group or organisation works",[]string{"business","organize","leadership"}},{"protect","Caring for nature or public wellbeing",[]string{"nature","science","care"}}}},
	}}
}

func assessmentForVersion(version string) CareerPathAssessment { if version == "careerpath-v1" { return careerPathAssessmentV1() }; return careerPathAssessment() }
func (s *CareerPathService) Assessment() CareerPathAssessment { return careerPathAssessment() }
func (s *CareerPathService) AssessmentForVersion(version string) CareerPathAssessment { return assessmentForVersion(version) }
func (s *CareerPathService) studentID(schoolID,userID uuid.UUID)(uuid.UUID,error){ var st models.Student; if err:=s.db.Where("school_id = ? AND user_id = ?",schoolID,userID).First(&st).Error;err!=nil {if errors.Is(err,gorm.ErrRecordNotFound){return uuid.Nil,ErrCareerPathForbidden};return uuid.Nil,err};return st.ID,nil }
func decodeResponses(raw string)(map[string][]string,error){ out:=map[string][]string{};if raw==""{return out,nil};if err:=json.Unmarshal([]byte(raw),&out);err!=nil{return nil,ErrCareerPathInvalidResponse};return out,nil }
func (s *CareerPathService) view(a models.CareerPathAttempt)(CareerPathAttemptView,error){ responses,e:=decodeResponses(a.Responses);if e!=nil{return CareerPathAttemptView{},e};v:=CareerPathAttemptView{ID:a.ID,Version:a.Version,Status:a.Status,Responses:responses,SubmittedAt:a.SubmittedAt,UpdatedAt:a.UpdatedAt};if a.Status=="submitted"{if e=json.Unmarshal([]byte(a.Results),&v.Results);e!=nil{return CareerPathAttemptView{},e}};return v,nil}
func (s *CareerPathService) getOwned(schoolID,userID,attemptID uuid.UUID)(models.CareerPathAttempt,error){studentID,e:=s.studentID(schoolID,userID);if e!=nil{return models.CareerPathAttempt{},e};var a models.CareerPathAttempt;e=s.db.Where("id = ? AND school_id = ? AND student_id = ? AND user_id = ?",attemptID,schoolID,studentID,userID).First(&a).Error;if errors.Is(e,gorm.ErrRecordNotFound){return a,ErrCareerPathNotFound};return a,e}
func (s *CareerPathService) CreateOrResume(schoolID,userID uuid.UUID)(CareerPathAttemptView,error){studentID,e:=s.studentID(schoolID,userID);if e!=nil{return CareerPathAttemptView{},e};var a models.CareerPathAttempt;e=s.db.Where("school_id = ? AND student_id = ? AND user_id = ? AND status = ?",schoolID,studentID,userID,"in_progress").Order("created_at DESC").First(&a).Error;if e==nil{return s.view(a)};if !errors.Is(e,gorm.ErrRecordNotFound){return CareerPathAttemptView{},e};
	// Keep a completed assessment visible when the student returns to CareerPath.
	// A new draft should not silently replace the results from the latest submission.
	e=s.db.Where("school_id = ? AND student_id = ? AND user_id = ? AND version = ? AND status = ?",schoolID,studentID,userID,CareerPathAssessmentVersion,"submitted").Order("submitted_at DESC").Order("created_at DESC").First(&a).Error
	if e==nil{return s.view(a)};if !errors.Is(e,gorm.ErrRecordNotFound){return CareerPathAttemptView{},e};a=models.CareerPathAttempt{SchoolID:schoolID,StudentID:studentID,UserID:userID,Version:CareerPathAssessmentVersion,Status:"in_progress",Responses:"{}",Results:"[]"};if e=s.db.Create(&a).Error;e!=nil{createErr:=e;var winner models.CareerPathAttempt;lookupErr:=s.db.Where("school_id = ? AND student_id = ? AND user_id = ? AND version = ? AND status = ?",schoolID,studentID,userID,CareerPathAssessmentVersion,"in_progress").Order("created_at DESC").First(&winner).Error;if lookupErr==nil{return s.view(winner)};return CareerPathAttemptView{},createErr};return s.view(a)}
func (s *CareerPathService) Get(schoolID,userID,attemptID uuid.UUID)(CareerPathAttemptView,error){a,e:=s.getOwned(schoolID,userID,attemptID);if e!=nil{return CareerPathAttemptView{},e};return s.view(a)}
func (s *CareerPathService) SaveResponses(schoolID,userID,attemptID uuid.UUID, incoming map[string][]string)(CareerPathAttemptView,error){a,e:=s.getOwned(schoolID,userID,attemptID);if e!=nil{return CareerPathAttemptView{},e};if a.Status!="in_progress"{return CareerPathAttemptView{},ErrCareerPathSubmitted};if a.Version!=CareerPathAssessmentVersion{return CareerPathAttemptView{},ErrCareerPathVersionMismatch};questions:=assessmentForVersion(a.Version).Questions;allowed:=map[string]map[string]bool{};for _,q:=range questions{allowed[q.ID]=map[string]bool{};for _,o:=range q.Options{allowed[q.ID][o.ID]=true}};for q,opts:=range incoming{valid,ok:=allowed[q];if !ok{return CareerPathAttemptView{},ErrCareerPathInvalidResponse};if !careerPathQuestionMultiple(a.Version,q)&&len(opts)>1{return CareerPathAttemptView{},ErrCareerPathInvalidResponse};seen:=map[string]bool{};for _,o:=range opts{if !valid[o]||seen[o]{return CareerPathAttemptView{},ErrCareerPathInvalidResponse};seen[o]=true}};existing,e:=decodeResponses(a.Responses);if e!=nil{return CareerPathAttemptView{},e};for q,opts:=range incoming{existing[q]=opts};raw,e:=json.Marshal(existing);if e!=nil{return CareerPathAttemptView{},e};res:=s.db.Model(&models.CareerPathAttempt{}).Where("id = ? AND school_id = ? AND student_id = ? AND status = ? AND updated_at = ?",a.ID,schoolID,a.StudentID,"in_progress",a.UpdatedAt).Updates(map[string]interface{}{"responses":string(raw)});if res.Error!=nil{return CareerPathAttemptView{},res.Error};if res.RowsAffected!=1{latest,readErr:=s.Get(schoolID,userID,attemptID);if readErr!=nil{return CareerPathAttemptView{},readErr};if latest.Status=="submitted"{return CareerPathAttemptView{},ErrCareerPathSubmitted};return CareerPathAttemptView{},ErrCareerPathConflict};return s.Get(schoolID,userID,attemptID)}
func careerPathQuestionMultiple(version,id string)bool{for _,q:=range assessmentForVersion(version).Questions{if q.ID==id{return q.Multiple}};return false}
func (s *CareerPathService) Submit(schoolID,userID,attemptID uuid.UUID)(CareerPathAttemptView,error){a,e:=s.getOwned(schoolID,userID,attemptID);if e!=nil{return CareerPathAttemptView{},e};if a.Status=="submitted"{return s.view(a)};if a.Version!=CareerPathAssessmentVersion{return CareerPathAttemptView{},ErrCareerPathVersionMismatch};responses,e:=decodeResponses(a.Responses);if e!=nil{return CareerPathAttemptView{},e};for _,q:=range assessmentForVersion(a.Version).Questions{if len(responses[q.ID])==0{return CareerPathAttemptView{},ErrCareerPathIncomplete}};selected:=map[string]bool{};tagLabels:=map[string]string{};for _,q:=range assessmentForVersion(a.Version).Questions{for _,o:=range q.Options{for _,answer:=range responses[q.ID]{if answer==o.ID{for _,tag:=range o.Tags{selected[tag]=true;tagLabels[tag]=strings.ToLower(o.Label)}}}}};matches:=[]CareerPathMatch{};for _,career:=range careerPathCatalogue(){shared:=[]string{};for _,tag:=range career.Tags{if selected[tag]{shared=append(shared,tag)}};if len(shared)==0{continue};reasons:=[]string{};for _,tag:=range shared{if label:=tagLabels[tag];label!=""{reasons=append(reasons,label)}};sort.Strings(reasons);matches=append(matches,CareerPathMatch{Career:career,MatchedInterests:reasons,SharedTags:len(shared),Explanation:"This option connects with interests or preferences you selected: "+strings.Join(reasons,", ")+". Explore it as one possibility, not a prediction."})};sort.Slice(matches,func(i,j int)bool{if matches[i].SharedTags==matches[j].SharedTags{return matches[i].Career.Title<matches[j].Career.Title};return matches[i].SharedTags>matches[j].SharedTags});if len(matches)>5{matches=matches[:5]};if len(matches)==0{return CareerPathAttemptView{},ErrCareerPathIncomplete};raw,e:=json.Marshal(matches);if e!=nil{return CareerPathAttemptView{},e};now:=time.Now().UTC();res:=s.db.Model(&models.CareerPathAttempt{}).Where("id = ? AND school_id = ? AND student_id = ? AND status = ? AND updated_at = ?",a.ID,schoolID,a.StudentID,"in_progress",a.UpdatedAt).Updates(map[string]interface{}{"results":string(raw),"status":"submitted","submitted_at":now});if res.Error!=nil{return CareerPathAttemptView{},res.Error};if res.RowsAffected==0{latest,readErr:=s.Get(schoolID,userID,attemptID);if readErr!=nil{return CareerPathAttemptView{},readErr};if latest.Status=="submitted"{return latest,nil};return CareerPathAttemptView{},ErrCareerPathConflict};return s.Get(schoolID,userID,attemptID)}
