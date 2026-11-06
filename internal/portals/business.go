package portals

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// A business portal is opened for an organization, so the user who will test it has to belong to that
// organization, with a connection to sign in with and a role. Setup asks for the user before it
// creates the portal, and checks each step before it changes anything.

const (
	businessRequirements = "Business portals only work in the context of an organization: without it, the organization " +
		"components do not load. Pick the user who will test the portal, so the link opens in their organization."
	skippedTestUserRequirements = "Without a test user, the link needs an organization. The portal only signs members in " +
		"when the URL carries ?organization=<id>."

	// The role setup creates when no role of the tenant has the permissions. It carries only the
	// organization permissions of the portal components, nothing more.
	portalRoleName = "Universal Portals - Admin"

	maxOrganizationNameLength = 50
)

// Organization names are identifiers: lowercase letters, numbers, hyphens and underscores, and
// unique in the tenant. The display name is free text.
var (
	organizationNamePattern = regexp.MustCompile(`^[a-z0-9_-]{1,50}$`)
	notOrganizationName     = regexp.MustCompile(`[^a-z0-9_-]+`)
)

// businessAccess is the access of the test user to an organization.
type businessAccess struct {
	userID           string
	organizationID   string
	organizationName string
	// Whether the roles of the user together grant every organization scope.
	hasOrganizationRole bool
}

// testLink is the organization the link of the portal opens in, and the access of the test user when
// one was chosen.
type testLink struct {
	organizationID string
	// True while the organization is still the placeholder to replace.
	isPlaceholder bool
	access        *businessAccess
}

func userLabel(user User) string {
	if user.Email != "" {
		return user.Email
	}

	return user.ID
}

func describeUser(user User) string {
	email := user.Email
	if email == "" {
		email = "no email"
	}

	description := email + " · " + user.ID
	if user.Name != "" {
		description += " · " + user.Name
	}

	return description
}

// addMemberCommand is the command that adds a user to an organization.
func addMemberCommand(organization, userID string) string {
	return fmt.Sprintf(`Add them: auth0 api post "organizations/%s/members" --data '{"members":["%s"]}'`, organization, userID)
}

// chooseTestLink settles the test user before anything is created, so a user who cannot sign in stops
// the run with nothing to clean up. Skipping the user asks for the organization ID of the link
// instead, with a placeholder when none is given.
func (r *setupRun) chooseTestLink(ctx context.Context) (testLink, error) {
	r.report().Section("Test user")

	userQuery := r.options.UserQuery

	if userQuery == "" && r.deps.Interactive {
		r.report().Info(businessRequirements)

		answer, err := r.deps.Prompter.Ask("Enter the test user's email or user ID (press Enter to skip)", "", "jane@acme.com")
		if err != nil {
			return testLink{}, err
		}

		userQuery = strings.TrimSpace(answer)
	}

	if userQuery != "" {
		access, err := r.resolveBusinessAccess(ctx, userQuery)
		if err != nil {
			return testLink{}, err
		}

		return testLink{organizationID: access.organizationID, access: &access}, nil
	}

	if r.deps.Interactive {
		r.report().Info(skippedTestUserRequirements)
	}

	organizationID, err := r.deps.Prompter.Ask(
		"Enter the organization ID to test in (the link adds ?organization=<id>)",
		organizationPlaceholder,
		"",
	)
	if err != nil {
		return testLink{}, err
	}

	return testLink{organizationID: organizationID, isPlaceholder: organizationID == organizationPlaceholder}, nil
}

func (r *setupRun) resolveBusinessAccess(ctx context.Context, userQuery string) (businessAccess, error) {
	user, err := r.chooseUser(ctx, userQuery)
	if err != nil {
		return businessAccess{}, err
	}

	r.report().Item(fmt.Sprintf("%q found", userLabel(user)), "")

	organization, err := r.chooseOrganization(ctx, user)
	if err != nil {
		return businessAccess{}, err
	}

	r.report().Item(fmt.Sprintf("Organization %q selected", organization.Name), "")

	if err := r.requireEnabledConnection(ctx, organization); err != nil {
		return businessAccess{}, err
	}

	hasRole, err := r.ensureOrganizationRole(ctx, user, organization)
	if err != nil {
		return businessAccess{}, err
	}

	return businessAccess{
		userID:              user.ID,
		organizationID:      organization.ID,
		organizationName:    organization.Name,
		hasOrganizationRole: hasRole,
	}, nil
}

// lookupUsers looks a user up by email, or by ID when the query has no "@". An unknown ID is an empty
// result and not an error, so the caller can ask again.
func (r *setupRun) lookupUsers(ctx context.Context, query string) ([]User, error) {
	if strings.Contains(query, "@") {
		found, err := r.deps.API.SearchUsersByEmail(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("search the users by email: %w", err)
		}

		// The search matches similar addresses too, so only the exact email is kept.
		return slices.DeleteFunc(found, func(user User) bool { return !strings.EqualFold(user.Email, query) }), nil
	}

	user, err := r.deps.API.ReadUser(ctx, query)
	if IsStatus(err, http.StatusNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read user %q: %w", query, err)
	}

	return []User{user}, nil
}

func (r *setupRun) chooseUser(ctx context.Context, initialQuery string) (User, error) {
	query := initialQuery

	for {
		if query == "" {
			if !r.deps.Interactive {
				return User{}, &BusinessAccessError{
					Message: "A Business portal needs the user who will test it",
					Hints:   []string{"Rerun with --user <email or user id>."},
				}
			}

			answer, err := r.deps.Prompter.Ask("Enter the test user's email or user ID", "", "jane@acme.com")
			if err != nil {
				return User{}, err
			}

			if query = strings.TrimSpace(answer); query == "" {
				continue
			}
		}

		matches, err := r.lookupUsers(ctx, query)
		if err != nil {
			return User{}, err
		}

		if len(matches) == 1 {
			return matches[0], nil
		}

		if !r.deps.Interactive {
			return User{}, noSingleUser(query, matches)
		}

		if len(matches) == 0 {
			r.report().Warn(fmt.Sprintf("No user matches %q.", query))

			query = ""

			continue
		}

		chosen, retry, err := r.pickUser(query, matches)
		if err != nil {
			return User{}, err
		}

		if retry {
			query = ""
			continue
		}

		return chosen, nil
	}
}

// noSingleUser is the error of a run that cannot ask which user is meant.
func noSingleUser(query string, matches []User) error {
	if len(matches) == 0 {
		return &BusinessAccessError{
			Message: fmt.Sprintf("No user matches %q", query),
			Hints:   []string{"Check the email or user ID, then rerun with --user."},
		}
	}

	ids := make([]string, 0, len(matches))
	for _, user := range matches {
		ids = append(ids, user.ID)
	}

	return &BusinessAccessError{
		Message: fmt.Sprintf("More than one user has the email %q", query),
		Hints:   []string{"Rerun with the user ID instead: " + strings.Join(ids, ", ")},
	}
}

// pickUser lets the person choose among the users that match, or look for another.
func (r *setupRun) pickUser(query string, matches []User) (user User, retry bool, err error) {
	options := make([]Choice, 0, len(matches)+1)
	for _, match := range matches {
		options = append(options, Choice{Label: describeUser(match)})
	}

	options = append(options, Choice{Label: "Try another email or user ID"})

	picked, err := r.deps.Prompter.Choose(fmt.Sprintf("%d users match %q", len(matches), query), options)
	if err != nil {
		return User{}, false, err
	}

	if picked == len(matches) {
		return User{}, true, nil
	}

	return matches[picked], false, nil
}

func (r *setupRun) chooseOrganization(ctx context.Context, user User) (Organization, error) {
	memberships, err := r.deps.API.ListUserOrganizations(ctx, user.ID)
	if err != nil {
		return Organization{}, fmt.Errorf("list the organizations of user %q: %w", user.ID, err)
	}

	if requested := r.options.Organization; requested != "" {
		for _, membership := range memberships {
			if membership.ID == requested || membership.Name == requested {
				return membership, nil
			}
		}

		return Organization{}, &BusinessAccessError{
			Message: fmt.Sprintf("%s is not a member of %q", userLabel(user), requested),
			Hints:   []string{addMemberCommand(requested, user.ID)},
		}
	}

	if !r.deps.Interactive {
		return nonInteractiveOrganization(user, memberships)
	}

	if len(memberships) == 1 {
		return memberships[0], nil
	}

	// Members pick one of their organizations. A user with none can join an existing organization of the
	// tenant. Either way a new one can be created.
	candidates := memberships

	if len(memberships) == 0 {
		candidates, err = r.deps.API.ListOrganizations(ctx)
		if err != nil {
			return Organization{}, fmt.Errorf("list the organizations: %w", err)
		}

		r.report().Warn(fmt.Sprintf("%s is not a member of any organization.", userLabel(user)))
	}

	r.report().Info("The portal opens in one organization, and the test user has to be a member of it.")

	options := make([]Choice, 0, len(candidates)+1)
	for _, candidate := range candidates {
		options = append(options, Choice{Label: fmt.Sprintf("%s (%s)", candidate.Name, candidate.ID)})
	}

	options = append(options, Choice{Label: "None of these, create a new organization"})

	picked, err := r.deps.Prompter.Choose("Choose the organization to test the portal in", options)
	if err != nil {
		return Organization{}, err
	}

	if picked == len(candidates) {
		return r.createOrganization(ctx, user)
	}

	if len(memberships) > 0 {
		return candidates[picked], nil
	}

	return r.addToOrganization(ctx, user, candidates[picked])
}

// nonInteractiveOrganization picks the organization when nobody can be asked.
func nonInteractiveOrganization(user User, memberships []Organization) (Organization, error) {
	switch len(memberships) {
	case 1:
		return memberships[0], nil
	case 0:
		return Organization{}, &BusinessAccessError{
			Message: fmt.Sprintf("%s is not a member of any organization", userLabel(user)),
			Hints:   []string{addMemberCommand("<org-id>", user.ID)},
		}
	default:
		ids := make([]string, 0, len(memberships))
		for _, membership := range memberships {
			ids = append(ids, membership.ID)
		}

		return Organization{}, &BusinessAccessError{
			Message: fmt.Sprintf("%s belongs to %d organizations", userLabel(user), len(memberships)),
			Hints:   []string{"Rerun with --organization <id>: " + strings.Join(ids, ", ")},
		}
	}
}

func (r *setupRun) addToOrganization(ctx context.Context, user User, organization Organization) (Organization, error) {
	confirmed, err := r.deps.Prompter.Confirm(fmt.Sprintf("Add %s as a member of %s?", userLabel(user), organization.Name), true)
	if err != nil {
		return Organization{}, err
	}

	if !confirmed {
		return Organization{}, &BusinessAccessError{
			Message: fmt.Sprintf("%s is not a member of an organization, and setup did not add them", userLabel(user)),
			Hints:   []string{addMemberCommand(organization.ID, user.ID)},
		}
	}

	if err := r.deps.API.AddOrganizationMember(ctx, organization.ID, user.ID); err != nil {
		return Organization{}, fmt.Errorf("add %s to organization %q: %w", userLabel(user), organization.Name, err)
	}

	r.report().Item(fmt.Sprintf("Added %s to %s", userLabel(user), organization.Name), "")

	return organization, nil
}

// suggestOrganizationName makes a name from the display name of an organization.
func suggestOrganizationName(displayName string) string {
	name := strings.Trim(notOrganizationName.ReplaceAllString(strings.ToLower(displayName), "-"), "-")

	if len(name) > maxOrganizationNameLength {
		name = name[:maxOrganizationNameLength]
	}

	return name
}

// createOrganization creates an organization, enables a database connection on it, and adds the
// user. Each write is confirmed first. A taken name asks again, as the slug of the portal does, with a
// suggestion that is free to use.
func (r *setupRun) createOrganization(ctx context.Context, user User) (Organization, error) {
	answer, err := r.deps.Prompter.Ask("Organization display name", "", "Acme Corp")
	if err != nil {
		return Organization{}, err
	}

	displayName := strings.TrimSpace(answer)
	if displayName == "" {
		return Organization{}, &BusinessAccessError{Message: "The organization needs a display name"}
	}

	suggestion := suggestOrganizationName(displayName)

	for {
		name, err := r.askOrganizationName(suggestion)
		if err != nil {
			return Organization{}, err
		}

		confirmed, err := r.deps.Prompter.Confirm(
			fmt.Sprintf("Create organization %q (%s), enable a database connection on it, and add %s?", displayName, name, userLabel(user)),
			true,
		)
		if err != nil {
			return Organization{}, err
		}

		if !confirmed {
			return Organization{}, &BusinessAccessError{
				Message: "No organization was created",
				Hints:   []string{fmt.Sprintf("Create one: auth0 orgs create --name %s --display %q", name, displayName)},
			}
		}

		created, err := r.deps.API.CreateOrganization(ctx, OrganizationSpec{Name: name, DisplayName: displayName})
		if IsStatus(err, http.StatusConflict) {
			// Only a taken name asks again. The later failures happen after the organization exists, so
			// asking for a new name would create a second one.
			r.report().Warn(fmt.Sprintf("The organization name %q is already used in this tenant.", name))

			suggestion = name + "-2"
			if len(suggestion) > maxOrganizationNameLength {
				suggestion = suggestion[:maxOrganizationNameLength]
			}

			continue
		}

		if err != nil {
			return Organization{}, fmt.Errorf("create organization %q: %w", name, err)
		}

		r.report().Item(fmt.Sprintf("Created organization %q", displayName), "")

		return created, r.connectOrganization(ctx, created, user, displayName)
	}
}

// connectOrganization enables a database connection on a new organization and adds the user to it.
func (r *setupRun) connectOrganization(ctx context.Context, organization Organization, user User, displayName string) error {
	connectionID, err := r.chooseDatabaseConnection(ctx)
	if err != nil {
		return err
	}

	if err := r.deps.API.EnableOrganizationConnection(ctx, organization.ID, connectionID); err != nil {
		return fmt.Errorf("enable the connection for organization %q: %w", organization.Name, err)
	}

	if err := r.deps.API.AddOrganizationMember(ctx, organization.ID, user.ID); err != nil {
		return fmt.Errorf("add %s to organization %q: %w", userLabel(user), organization.Name, err)
	}

	r.report().Item(fmt.Sprintf("Added %s to %s", userLabel(user), displayName), "")

	return nil
}

func (r *setupRun) askOrganizationName(suggestion string) (string, error) {
	for {
		answer, err := r.deps.Prompter.Ask("Organization name", suggestion, "")
		if err != nil {
			return "", err
		}

		if name := strings.TrimSpace(answer); organizationNamePattern.MatchString(name) {
			return name, nil
		}

		r.report().Error("Use lowercase letters, numbers, hyphens and underscores, up to 50 characters.")
	}
}

func (r *setupRun) chooseDatabaseConnection(ctx context.Context) (string, error) {
	connections, err := r.deps.API.ListDatabaseConnections(ctx)
	if err != nil {
		return "", fmt.Errorf("list the database connections: %w", err)
	}

	switch len(connections) {
	case 0:
		return "", &BusinessAccessError{
			Message: "This tenant has no database connection to sign in with",
			Hints:   []string{"Create one in the dashboard, then rerun setup."},
		}
	case 1:
		return connections[0].ID, nil
	}

	options := make([]Choice, 0, len(connections))
	for _, connection := range connections {
		options = append(options, Choice{Label: connection.Name})
	}

	picked, err := r.deps.Prompter.Choose("Database connection for the organization", options)
	if err != nil {
		return "", err
	}

	return connections[picked].ID, nil
}

func (r *setupRun) requireEnabledConnection(ctx context.Context, organization Organization) error {
	connections, err := r.deps.API.ListOrganizationConnections(ctx, organization.ID)
	if err != nil {
		return fmt.Errorf("list the connections of organization %q: %w", organization.Name, err)
	}

	if len(connections) > 0 {
		return nil
	}

	return &BusinessAccessError{
		Message: fmt.Sprintf("Organization %q has no enabled connections, so members cannot sign in", organization.Name),
		Hints: []string{fmt.Sprintf(
			`Enable one: auth0 api post "organizations/%s/enabled_connections" --data '{"connection_id":"<connection-id>","assign_membership_on_login":false}'`,
			organization.ID,
		)},
	}
}

// ensureOrganizationRole returns whether the organization roles of the user, together, grant every
// scope the organization components need. Missing scopes are a warning and not a stop: the portal
// still signs the user in, but those components do not work for them.
func (r *setupRun) ensureOrganizationRole(ctx context.Context, user User, organization Organization) (bool, error) {
	roles, err := r.deps.API.ListMemberRoles(ctx, organization.ID, user.ID)
	if err != nil {
		return false, fmt.Errorf("list the roles of %s in organization %q: %w", userLabel(user), organization.Name, err)
	}

	granted := map[string]bool{}

	for _, role := range roles {
		scopes, err := r.permissionsOf(ctx, role.ID)
		if err != nil {
			return false, err
		}

		for _, scope := range scopes {
			granted[scope] = true
		}
	}

	var missing []string

	for _, scope := range myOrganizationScopes {
		if !granted[scope] {
			missing = append(missing, scope)
		}
	}

	if len(missing) == 0 {
		return true, nil
	}

	r.warnAboutMissingPermissions(user, organization, missing)

	if !r.deps.Interactive {
		return false, nil
	}

	return r.offerRole(ctx, user, organization, missing)
}

func (r *setupRun) warnAboutMissingPermissions(user User, organization Organization, missing []string) {
	var reads, writes []string

	for _, scope := range missing {
		if strings.HasPrefix(scope, "read:") {
			reads = append(reads, scope)
		} else {
			writes = append(writes, scope)
		}
	}

	if len(reads) > 0 {
		r.report().Warn(fmt.Sprintf(
			"%s lacks read permissions for the organization components in %q: %s. Those components will not open for them.",
			userLabel(user), organization.Name, strings.Join(reads, ", "),
		))
	}

	if len(writes) > 0 {
		r.report().Warn(fmt.Sprintf(
			"%s lacks write permissions for the organization components in %q: %s. The components open, but changes cannot be saved.",
			userLabel(user), organization.Name, strings.Join(writes, ", "),
		))
	}
}

// offerRole offers to give the user a role that grants the missing permissions.
func (r *setupRun) offerRole(ctx context.Context, user User, organization Organization, missing []string) (bool, error) {
	candidates, err := r.rolesGranting(ctx, missing)
	if err != nil {
		return false, err
	}

	var role Role

	if len(candidates) == 0 {
		create, err := r.deps.Prompter.Confirm(
			fmt.Sprintf("No role in this tenant has these permissions. Create %q and assign it to %s?", portalRoleName, userLabel(user)),
			true,
		)
		if err != nil || !create {
			return false, err
		}

		if role, err = r.createPortalRole(ctx); err != nil {
			return false, err
		}
	} else {
		assign, err := r.deps.Prompter.Confirm("Assign an organization role now?", true)
		if err != nil || !assign {
			return false, err
		}

		options := make([]Choice, 0, len(candidates)+1)
		for _, candidate := range candidates {
			options = append(options, Choice{Label: candidate.Name})
		}

		options = append(options, Choice{Label: fmt.Sprintf("Create %q", portalRoleName), Hint: "only the permissions the portal needs"})

		picked, err := r.deps.Prompter.Choose("Role to assign", options)
		if err != nil {
			return false, err
		}

		if picked == len(candidates) {
			role, err = r.createPortalRole(ctx)
		} else {
			role = candidates[picked]
		}

		if err != nil {
			return false, err
		}
	}

	if err := r.deps.API.AssignMemberRole(ctx, organization.ID, user.ID, role.ID); err != nil {
		return false, fmt.Errorf("assign role %q to %s: %w", role.Name, userLabel(user), err)
	}

	r.report().Item(fmt.Sprintf("Assigned role %s to %s", role.Name, userLabel(user)), "")

	return true, nil
}

// createPortalRole creates the role of the portal components. A role with the same name is reused, and
// only the missing permissions are added to it.
func (r *setupRun) createPortalRole(ctx context.Context) (Role, error) {
	tenantRoles, err := r.deps.API.ListRoles(ctx)
	if err != nil {
		return Role{}, fmt.Errorf("list the roles: %w", err)
	}

	var role Role

	if index := slices.IndexFunc(tenantRoles, func(candidate Role) bool { return candidate.Name == portalRoleName }); index >= 0 {
		role = tenantRoles[index]
	} else {
		role, err = r.deps.API.CreateRole(ctx, RoleSpec{
			Name:        portalRoleName,
			Description: "The organization permissions the Universal Portals components need.",
		})
		if err != nil {
			return Role{}, fmt.Errorf("create role %q: %w", portalRoleName, err)
		}
	}

	granted, err := r.permissionsOf(ctx, role.ID)
	if err != nil {
		return Role{}, err
	}

	var missing []Permission

	for _, scope := range myOrganizationScopes {
		if !slices.Contains(granted, scope) {
			missing = append(missing, Permission{Name: scope, ResourceServerIdentifier: myOrganizationAudience(r.tenant)})
		}
	}

	if len(missing) > 0 {
		if err := r.deps.API.AddRolePermissions(ctx, role.ID, missing); err != nil {
			return Role{}, fmt.Errorf("add the permissions to role %q: %w", role.Name, err)
		}
	}

	return role, nil
}

func (r *setupRun) permissionsOf(ctx context.Context, roleID string) ([]string, error) {
	permissions, err := r.deps.API.ListRolePermissions(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("list the permissions of role %q: %w", roleID, err)
	}

	names := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		names = append(names, permission.Name)
	}

	return names, nil
}

// rolesGranting returns the roles of the tenant that grant every one of the scopes, so assigning one
// fixes the warning.
func (r *setupRun) rolesGranting(ctx context.Context, scopes []string) ([]Role, error) {
	tenantRoles, err := r.deps.API.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the roles: %w", err)
	}

	var matches []Role

	for _, role := range tenantRoles {
		granted, err := r.permissionsOf(ctx, role.ID)
		if err != nil {
			return nil, err
		}

		if !slices.ContainsFunc(scopes, func(scope string) bool { return !slices.Contains(granted, scope) }) {
			matches = append(matches, role)
		}
	}

	return matches, nil
}
