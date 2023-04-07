package cli

/*
func TestUserRolesToRemovePickerOptions(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		roles        []*management.Role
		apiError     error
		assertOutput func(t testing.TB, options []string)
		assertError  func(t testing.TB, err error)
	}{
		{
			name: "happy path",
			roles: []*management.Role{
				{
					ID:   auth0.String("some-id-1"),
					Name: auth0.String("some-name-1"),
				},
				{
					ID:   auth0.String("some-id-2"),
					Name: auth0.String("some-name-2"),
				},
			},
			assertOutput: func(t testing.TB, options []string) {
				assert.Len(t, options, 2)
				assert.Equal(t, "some-name-1 (some-id-1)", options[0].label)
				assert.Equal(t, "some-id-1", options[0].value)
				assert.Equal(t, "some-name-2 (some-id-2)", options[1].label)
				assert.Equal(t, "some-id-2", options[1].value)
			},
			assertError: func(t testing.TB, err error) {
				t.Fail()
			},
		},
		{
			name:  "no roles",
			roles: []*management.Role{},
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.ErrorContains(t, err, "There are currently no roles.")
			},
		},
		{
			name:     "API error",
			apiError: errors.New("error"),
			assertOutput: func(t testing.TB, options []string) {
				t.Fail()
			},
			assertError: func(t testing.TB, err error) {
				assert.Error(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			userAPI := mock.NewMockUserAPI(ctrl)
			userAPI.EXPECT().
				Roles(gomock.Any()).
				Return(&management.RoleList{
					Roles: test.roles}, test.apiError)

		    // mock roles API and add to cli

			cli := &cli{
				api: &auth0.API{User: userAPI},
			}

			options, err := userRolesToRemovePickerOptions(cli, test.userID)

			if err != nil {
				test.assertError(t, err)
			} else {
				test.assertOutput(t, options)
			}
		})
	}
}

*/