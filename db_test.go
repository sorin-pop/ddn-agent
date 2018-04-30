package main

import "testing"

var (
	passingCases = []string{"mysql", "mariadb", "oracle", "postgres", "MySQL", "Oracle", "PostgrEs"}
	failingCases = []string{"asd", "mysqla", "posgres"}
)

func TestVendorSupported(t *testing.T) {

	for _, v := range passingCases {
		if err := VendorSupported(v); err != nil {
			t.Errorf("Error; Should have passed, but failed for %q.", v)
		}
	}

	for _, v := range failingCases {
		if err := VendorSupported(v); err == nil {
			t.Errorf("Error; Should have failed, but passed for %q.", v)
		}
	}
}

func TestGetDB(t *testing.T) {
	for _, v := range passingCases {
		_, err := GetDB(v)
		if err != nil {
			t.Errorf("Error; Should have passed, but failed for %q with message %q", v, err.Error())
		}
	}

	for _, v := range failingCases {
		_, err := GetDB(v)
		if err == nil {
			t.Errorf("Error; Should have failed, but passed for %q", v)
		}
	}
}

func TestGetDB_Two(t *testing.T) {
	var (
		test string
		db   Database
		err  error
	)

	test = "mysql"
	db, err = GetDB(test)
	if err != nil {
		t.Errorf("Error; Should have passed, but failed for %q with message %q", test, err.Error())
	}

	if _, ok := db.(*mysql); !ok {
		t.Errorf("Error; Type assertion should have passed, but failed for %q", test)
	}

	test = "oracle"
	db, err = GetDB(test)
	if err != nil {
		t.Errorf("Error; Should have passed, but failed for %q with message %q", test, err.Error())
	}

	if _, ok := db.(*oracle); !ok {
		t.Errorf("Error; Type assertion should have passed, but failed for %q", test)
	}

	test = "postgres"
	db, err = GetDB(test)
	if err != nil {
		t.Errorf("Error; Should have passed, but failed for %q with message %q", test, err.Error())
	}

	if _, ok := db.(*postgres); !ok {
		t.Errorf("Error; Type assertion should have passed, but failed for %q", test)
	}

	test = "fails"
	db, err = GetDB(test)
	if err == nil {
		t.Errorf("Error; Should have failed, but passed for %q with message %q", test, err.Error())
	}

	if _, ok := db.(*mysql); ok {
		t.Errorf("Error; Type assertion should have failed, but passed for %q", test)
	}

}
