package database

import (
    "fmt"
    "strconv"
    "strings"

    "gorm.io/gorm"
)

type nationalCurriculumSeedSubject struct {
    code string
    name string
    category string
    required bool
    selectionGroup *string
}

func parseNationalCurriculumSubjects(spec string) []nationalCurriculumSeedSubject {
    var result []nationalCurriculumSeedSubject
    for _, item := range strings.Split(spec, "|") {
        parts := strings.Split(item, ",")
        if len(parts) != 5 {
            continue
        }
        required, _ := strconv.ParseBool(parts[3])
        var group *string
        if parts[4] != "" {
            value := parts[4]
            group = &value
        }
        result = append(result, nationalCurriculumSeedSubject{code: parts[0], name: parts[1], category: parts[2], required: required, selectionGroup: group})
    }
    return result
}

func seedNigeriaNationalCurriculum(tx *gorm.DB) error {
    type curriculum struct { code, name, version, description string }
    curricula := []curriculum{
        {"NG-NERDC-BEC-2025", "Nigeria National Curriculum — Revised Basic Education Curriculum", "2025 Revised BEC", "NERDC revised 9-Year Basic Education Curriculum for Primary 1-6 and JSS 1-3. Phased implementation begins September 2025."},
        {"NG-NERDC-SSEC-2025", "Nigeria National Curriculum — Revised Senior Secondary Education Curriculum", "2025 Revised SSEC", "NERDC revised Senior Secondary Education Curriculum for SS 1-3. Phased implementation begins September 2025."},
    }
    curriculumIDs := map[string]string{}
    for _, c := range curricula {
        var id string
        err := tx.Raw(`INSERT INTO national_curricula
            (code,name,authority,version,description,effective_from,status,created_at,updated_at)
            VALUES (?, ?, 'NERDC', ?, ?, '2025-09-01T00:00:00Z', 'active', NOW(), NOW())
            ON CONFLICT (code) DO UPDATE SET
                name=EXCLUDED.name, authority=EXCLUDED.authority, version=EXCLUDED.version,
                description=EXCLUDED.description, effective_from=EXCLUDED.effective_from,
                status='active', updated_at=NOW()
            RETURNING id`, c.code, c.name, c.version, c.description).Scan(&id).Error
        if err != nil { return fmt.Errorf("seed curriculum %s: %w", c.code, err) }
        curriculumIDs[c.code] = id
    }

    type level struct { curriculumCode, code, name string; order int }
    levels := []level{
        {"NG-NERDC-BEC-2025", "PRIMARY", "Primary Education", 1},
        {"NG-NERDC-BEC-2025", "JSS", "Junior Secondary School", 2},
        {"NG-NERDC-SSEC-2025", "SSS", "Senior Secondary School", 1},
    }
    levelIDs := map[string]string{}
    for _, l := range levels {
        var id string
        err := tx.Raw(`INSERT INTO national_curriculum_levels
            (curriculum_id,code,name,display_order,created_at,updated_at)
            VALUES (?, ?, ?, ?, NOW(), NOW())
            ON CONFLICT (curriculum_id,code) DO UPDATE SET
                name=EXCLUDED.name, display_order=EXCLUDED.display_order, updated_at=NOW()
            RETURNING id`, curriculumIDs[l.curriculumCode], l.code, l.name, l.order).Scan(&id).Error
        if err != nil { return fmt.Errorf("seed level %s: %w", l.code, err) }
        levelIDs[l.curriculumCode+":"+l.code] = id
    }

    const primary13 = "ENG-STUDIES,English Studies,core,1,|MATH,Mathematics,core,1,|NIG-LANG,Nigerian Languages (One Nigerian Language),core,1,nigerian_language|BASIC-SCI,Basic Science,core,1,|PHE,Physical & Health Education,core,1,|RELIGION,CRS / Islamic Studies,religious,0,religious_studies|NIG-HIST,Nigerian History,core,1,|SOCIAL-CIT,Social and Citizenship Studies,core,1,|CCA,Cultural & Creative Arts (CCA),core,1,|ARABIC,Arabic Language,optional,0,"
    const primary46 = "ENG-STUDIES,English Studies,core,1,|MATH,Mathematics,core,1,|NIG-LANG,Nigerian Languages (One Nigerian Language),core,1,nigerian_language|BASIC-ST,Basic Science and Technology,core,1,|PHE,Physical & Health Education,core,1,|BASIC-DIGITAL,Basic Digital Literacy,core,1,|RELIGION,CRS / Islamic Studies,religious,0,religious_studies|NIG-HIST,Nigerian History,core,1,|SOCIAL-CIT,Social and Citizenship Studies,core,1,|CCA,Cultural & Creative Arts (CCA),core,1,|PREVOC,Pre-vocational Studies,core,1,|FRENCH,French,optional,0,|ARABIC,Arabic Language,optional,0,"
    const jss = "ENG-STUDIES,English Studies,core,1,|MATH,Mathematics,core,1,|NIG-LANG,Nigerian Languages (One Nigerian Language),core,1,nigerian_language|INTER-SCI,Intermediate Science,core,1,|PHE,Physical & Health Education,core,1,|DIGITAL-TECH,Digital Technologies,core,1,|CRS,Christian Religious Studies (CRS),religious,0,religious_studies|IS,Islamic Studies (IS),religious,0,religious_studies|NIG-HIST,Nigerian History,core,1,|SOCIAL-CIT,Social and Citizenship Studies,core,1,|CCA,Cultural & Creative Arts (CCA),core,1,|TRADE-SOLAR,Solar Photovoltaic Installation and Maintenance,trade,0,trade_subject|TRADE-FASHION,Fashion Design and Garment Making,trade,0,trade_subject|TRADE-LIVESTOCK,Livestock Farming,trade,0,trade_subject|TRADE-BEAUTY,Beauty and Cosmetology,trade,0,trade_subject|TRADE-HARDWARE,Computer Hardware and GSM Repairs,trade,0,trade_subject|TRADE-HORTICULTURE,Horticulture and Crop Production,trade,0,trade_subject|BUSINESS,Business Studies,core,1,|FRENCH,French,optional,0,|ARABIC,Arabic Language,optional,0,"
    const senior = "ENG-LANG,English Language,core,1,|GEN-MATH,General Mathematics,core,1,|CIT-HERITAGE,Citizenship and Heritage Studies,core,1,|DIGITAL-TECH,Digital Technologies,core,1,|TRADE-SOLAR,Solar Photovoltaic Installation and Maintenance,trade,0,ss_core_trade|TRADE-FASHION,Fashion Design and Garment Making,trade,0,ss_core_trade|TRADE-LIVESTOCK,Livestock Farming,trade,0,ss_core_trade|TRADE-BEAUTY,Beauty and Cosmetology,trade,0,ss_core_trade|TRADE-HARDWARE,Computer Hardware and GSM Repairs,trade,0,ss_core_trade|TRADE-HORTICULTURE,Horticulture and Crop Production,trade,0,ss_core_trade|BIOLOGY,Biology,optional,0,science|CHEMISTRY,Chemistry,optional,0,science|PHYSICS,Physics,optional,0,science|AGRICULTURE,Agriculture,optional,0,science|FURTHER-MATH,Further Mathematics,optional,0,science|PHYSICAL-EDU,Physical Education,optional,0,science|HEALTH-EDU,Health Education,optional,0,science|FOODS-NUTRITION,Foods & Nutrition,optional,0,science|GEOGRAPHY,Geography,optional,0,science|TECH-DRAWING,Technical Drawing,optional,0,science|NIG-HIST,Nigerian History,optional,0,humanities|GOVERNMENT,Government,optional,0,humanities|CRS,Christian Religious Studies,religious,0,humanities_religious|IS,Islamic Studies,religious,0,humanities_religious|NIG-LANG-HAUSA,Hausa,optional,0,ss_nigerian_language|NIG-LANG-IGBO,Igbo,optional,0,ss_nigerian_language|NIG-LANG-YORUBA,Yoruba,optional,0,ss_nigerian_language|FRENCH,French,optional,0,humanities|ARABIC,Arabic,optional,0,humanities|VISUAL-ARTS,Visual Arts,optional,0,humanities|MUSIC,Music,optional,0,humanities|LIT-ENGLISH,Literature in English,optional,0,humanities|HOME-MGMT,Home Management,optional,0,humanities|CATERING-CRAFT,Catering Craft,optional,0,humanities|ACCOUNTING,Accounting,optional,0,business|COMMERCE,Commerce,optional,0,business|MARKETING,Marketing,optional,0,business|ECONOMICS,Economics,optional,0,business"

    type classSeed struct {
        curriculumCode, levelCode, code, name, effectiveFrom string
        order int
        subjects []nationalCurriculumSeedSubject
    }
    classes := []classSeed{
        {"NG-NERDC-BEC-2025","PRIMARY","P1","Primary 1","2025-09-01",1,parseNationalCurriculumSubjects(primary13)},
        {"NG-NERDC-BEC-2025","PRIMARY","P2","Primary 2","2026-09-01",2,parseNationalCurriculumSubjects(primary13)},
        {"NG-NERDC-BEC-2025","PRIMARY","P3","Primary 3","2027-09-01",3,parseNationalCurriculumSubjects(primary13)},
        {"NG-NERDC-BEC-2025","PRIMARY","P4","Primary 4","2025-09-01",4,parseNationalCurriculumSubjects(primary46)},
        {"NG-NERDC-BEC-2025","PRIMARY","P5","Primary 5","2026-09-01",5,parseNationalCurriculumSubjects(primary46)},
        {"NG-NERDC-BEC-2025","PRIMARY","P6","Primary 6","2027-09-01",6,parseNationalCurriculumSubjects(primary46)},
        {"NG-NERDC-BEC-2025","JSS","JSS1","JSS 1","2025-09-01",1,parseNationalCurriculumSubjects(jss)},
        {"NG-NERDC-BEC-2025","JSS","JSS2","JSS 2","2026-09-01",2,parseNationalCurriculumSubjects(jss)},
        {"NG-NERDC-BEC-2025","JSS","JSS3","JSS 3","2027-09-01",3,parseNationalCurriculumSubjects(jss)},
        {"NG-NERDC-SSEC-2025","SSS","SS1","SS 1","2025-09-01",1,parseNationalCurriculumSubjects(senior)},
        {"NG-NERDC-SSEC-2025","SSS","SS2","SS 2","2026-09-01",2,parseNationalCurriculumSubjects(senior)},
        {"NG-NERDC-SSEC-2025","SSS","SS3","SS 3","2027-09-01",3,parseNationalCurriculumSubjects(senior)},
    }

    for _, c := range classes {
        levelID := levelIDs[c.curriculumCode+":"+c.levelCode]
        if levelID == "" { return fmt.Errorf("missing level for %s/%s", c.curriculumCode, c.levelCode) }
        var classID string
        err := tx.Raw(`INSERT INTO national_curriculum_classes
            (level_id,code,name,display_order,created_at,updated_at)
            VALUES (?, ?, ?, ?, NOW(), NOW())
            ON CONFLICT (level_id,code) DO UPDATE SET
                name=EXCLUDED.name, display_order=EXCLUDED.display_order, updated_at=NOW()
            RETURNING id`, levelID, c.code, c.name, c.order).Scan(&classID).Error
        if err != nil { return fmt.Errorf("seed class %s: %w", c.code, err) }

        for _, s := range c.subjects {
            if err := tx.Exec(`INSERT INTO national_curriculum_subjects
                (class_id,code,name,category,required,selection_group,effective_from,active,created_at,updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, true, NOW(), NOW())
                ON CONFLICT (class_id,code) DO UPDATE SET
                    name=EXCLUDED.name, category=EXCLUDED.category, required=EXCLUDED.required,
                    selection_group=EXCLUDED.selection_group, effective_from=EXCLUDED.effective_from,
                    active=true, updated_at=NOW()`,
                classID, s.code, s.name, s.category, s.required, s.selectionGroup, c.effectiveFrom).Error; err != nil {
                return fmt.Errorf("seed subject %s for %s: %w", s.code, c.code, err)
            }
        }
    }
    return nil
}
