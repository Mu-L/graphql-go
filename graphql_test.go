package graphql_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/example/social"
	"github.com/graph-gophers/graphql-go/example/starwars"
	"github.com/graph-gophers/graphql-go/gqltesting"
	"github.com/graph-gophers/graphql-go/introspection"
	"github.com/graph-gophers/graphql-go/trace/tracer"
)

type helloWorldResolver1 struct{}

func (r *helloWorldResolver1) Hello() string {
	return "Hello world!"
}

type helloWorldResolver2 struct{}

func (r *helloWorldResolver2) Hello(ctx context.Context) (string, error) {
	return "Hello world!", nil
}

type helloSnakeResolver1 struct{}

func (r *helloSnakeResolver1) HelloHTML() string {
	return "Hello snake!"
}

func (r *helloSnakeResolver1) SayHello(args struct{ FullName string }) string {
	return "Hello " + args.FullName + "!"
}

type helloSnakeResolver2 struct{}

func (r *helloSnakeResolver2) HelloHTML(ctx context.Context) (string, error) {
	return "Hello snake!", nil
}

func (r *helloSnakeResolver2) SayHello(ctx context.Context, args struct{ FullName string }) (string, error) {
	return "Hello " + args.FullName + "!", nil
}

type structFieldResolver struct {
	Hello string
}

type theNumberResolver struct {
	number int32
}

func (r *theNumberResolver) TheNumber() int32 {
	return r.number
}

func (r *theNumberResolver) ChangeTheNumber(args struct{ NewNumber int32 }) *theNumberResolver {
	r.number = args.NewNumber
	return r
}

type timeResolver struct{}

func (r *timeResolver) AddHour(args struct{ Time graphql.Time }) graphql.Time {
	return graphql.Time{Time: args.Time.Add(time.Hour)}
}

type echoResolver struct{}

func (r *echoResolver) Echo(args struct{ Value *string }) *string {
	return args.Value
}

var starwarsSchema = graphql.MustParseSchema(starwars.Schema, &starwars.Resolver{})

type ResolverError interface {
	error
	Extensions() map[string]interface{}
}

type resolverNotFoundError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e resolverNotFoundError) Error() string {
	return fmt.Sprintf("Error [%s]: %s", e.Code, e.Message)
}

func (e resolverNotFoundError) Extensions() map[string]interface{} {
	return map[string]interface{}{
		"code":    e.Code,
		"message": e.Message,
	}
}

var (
	droidNotFoundError = resolverNotFoundError{
		Code:    "NotFound",
		Message: "This is not the droid you are looking for",
	}
	errQuote = errors.New("bleep bloop")

	r2d2          = &droidResolver{name: "R2-D2"}
	c3po          = &droidResolver{name: "C-3PO"}
	notFoundDroid = &droidResolver{err: droidNotFoundError}
)

type findDroidsResolver struct{}

func (r *findDroidsResolver) FindDroids(ctx context.Context) []*droidResolver {
	return []*droidResolver{r2d2, notFoundDroid, c3po}
}

func (r *findDroidsResolver) FindNilDroids(ctx context.Context) *[]*droidResolver {
	return &[]*droidResolver{r2d2, nil, c3po}
}

type findDroidOrHumanResolver struct{}

func (r *findDroidOrHumanResolver) FindHuman(ctx context.Context) (*string, error) {
	human := "human"
	return &human, nil
}

func (r *findDroidOrHumanResolver) FindDroid(ctx context.Context) (*droidResolver, error) {
	return nil, notFoundDroid.err
}

type droidResolver struct {
	name string
	err  error
}

func (d *droidResolver) Name() (string, error) {
	if d.err != nil {
		return "", d.err
	}
	return d.name, nil
}

func (d *droidResolver) Quotes() ([]string, error) {
	switch d.name {
	case r2d2.name:
		return nil, errQuote
	case c3po.name:
		return []string{"We're doomed!", "R2-D2, where are you?"}, nil
	}
	return nil, nil
}

type discussPlanResolver struct{}

func (r *discussPlanResolver) DismissVader(ctx context.Context) (string, error) {
	return "", errors.New("I find your lack of faith disturbing")
}

func TestHelloWorld(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					hello: String!
				}
			`, &helloWorldResolver1{}),
			Query: `
				{
					hello
				}
			`,
			ExpectedResult: `
				{
					"hello": "Hello world!"
				}
			`,
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					hello: String!
				}
			`, &helloWorldResolver2{}),
			Query: `
				{
					hello
				}
			`,
			ExpectedResult: `
				{
					"hello": "Hello world!"
				}
			`,
		},
	})
}

func TestHelloWorldStructFieldResolver(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					hello: String!
				}
			`,
				&structFieldResolver{Hello: "Hello world!"},
				graphql.UseFieldResolvers()),
			Query: `
				{
					hello
				}
			`,
			ExpectedResult: `
				{
					"hello": "Hello world!"
				}
			`,
		},
	})
}

func TestHelloSnake(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					hello_html: String!
				}
			`, &helloSnakeResolver1{}),
			Query: `
				{
					hello_html
				}
			`,
			ExpectedResult: `
				{
					"hello_html": "Hello snake!"
				}
			`,
		},

		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					hello_html: String!
				}
			`, &helloSnakeResolver2{}),
			Query: `
				{
					hello_html
				}
			`,
			ExpectedResult: `
				{
					"hello_html": "Hello snake!"
				}
			`,
		},
	})
}

func TestHelloSnakeArguments(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					say_hello(full_name: String!): String!
				}
			`, &helloSnakeResolver1{}),
			Query: `
				{
					say_hello(full_name: "Rob Pike")
				}
			`,
			ExpectedResult: `
				{
					"say_hello": "Hello Rob Pike!"
				}
			`,
		},

		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					say_hello(full_name: String!): String!
				}
			`, &helloSnakeResolver2{}),
			Query: `
				{
					say_hello(full_name: "Rob Pike")
				}
			`,
			ExpectedResult: `
				{
					"say_hello": "Hello Rob Pike!"
				}
			`,
		},
	})
}

func TestRootOperations_invalidSchema(t *testing.T) {
	type args struct {
		Schema string
	}
	type want struct {
		Error string
	}
	testTable := map[string]struct {
		Args args
		Want want
	}{
		"Empty schema": {
			Want: want{Error: `root operation "query" must be defined`},
		},
		"Query declared by schema, but type not present": {
			Args: args{
				Schema: `
					schema {
						query: Query
					}
				`,
			},
			Want: want{Error: `graphql: type "Query" not found`},
		},
		"Query as incorrect type": {
			Args: args{
				Schema: `
					schema {
						query: String
					}
				`,
			},
			Want: want{Error: `root operation "query" must be an OBJECT`},
		},
		"Query with custom name, schema omitted": {
			Args: args{
				Schema: `
					type QueryType {
						hello: String!
					}
				`,
			},
			Want: want{Error: `root operation "query" must be defined`},
		},
		"Mutation as incorrect type": {
			Args: args{
				Schema: `
					schema {
						query: Query
						mutation: String
					}
					type Query {
						thing: String
					}
				`,
			},
			Want: want{Error: `root operation "mutation" must be an OBJECT`},
		},
		"Mutation declared by schema, but type not present": {
			Args: args{
				Schema: `
					schema {
						query: Query
						mutation: Mutation
					}
					type Query {
						hello: String!
					}
				`,
			},
			Want: want{Error: `graphql: type "Mutation" not found`},
		},
	}

	for name, tt := range testTable {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := graphql.ParseSchema(tt.Args.Schema, nil)
			if err == nil || err.Error() != tt.Want.Error {
				t.Logf("got:  %v", err)
				t.Logf("want: %s", tt.Want.Error)
				t.Fail()
			}
		})
	}
}

func TestRootOperations_validSchema(t *testing.T) {
	type resolver struct {
		helloSaidResolver
		helloWorldResolver1
		theNumberResolver
	}
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			// Query only, default name with `schema` omitted
			Schema: graphql.MustParseSchema(`
				type Query {
					hello: String!
				}
			`, &resolver{}),
			Query:          `{ hello }`,
			ExpectedResult: `{"hello": "Hello world!"}`,
		},
		{
			// Query only, default name with `schema` present
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}
				type Query {
					hello: String!
				}
			`, &resolver{}),
			Query:          `{ hello }`,
			ExpectedResult: `{"hello": "Hello world!"}`,
		},
		{
			// Query only, custom name
			Schema: graphql.MustParseSchema(`
				schema {
					query: QueryType
				}
				type QueryType {
					hello: String!
				}
			`, &resolver{}),
			Query:          `{ hello }`,
			ExpectedResult: `{"hello": "Hello world!"}`,
		},
		{
			// Query+Mutation, default names with `schema` omitted
			Schema: graphql.MustParseSchema(`
				type Query {
					hello: String!
				}
				type Mutation {
					changeTheNumber(newNumber: Int!): ChangedNumber!
				}
				type ChangedNumber {
					theNumber: Int!
				}
			`, &resolver{}),
			Query: `
				mutation {
					changeTheNumber(newNumber: 1) {
						theNumber
					}
				}
			`,
			ExpectedResult: `{"changeTheNumber": {"theNumber": 1}}`,
		},
		{
			// Query+Mutation, custom names
			Schema: graphql.MustParseSchema(`
				schema {
					query: QueryType
					mutation: MutationType
				}
				type QueryType {
					hello: String!
				}
				type MutationType {
					changeTheNumber(newNumber: Int!): ChangedNumber!
				}
				type ChangedNumber {
					theNumber: Int!
				}
			`, &resolver{}),
			Query: `
				mutation {
					changeTheNumber(newNumber: 1) {
						theNumber
					}
				}
			`,
			ExpectedResult: `{"changeTheNumber": {"theNumber": 1}}`,
		},
		{
			// Mutation with custom name, schema omitted
			Schema: graphql.MustParseSchema(`
				type Query {
					hello: String!
				}
				type MutationType {
					changeTheNumber(newNumber: Int!): ChangedNumber!
				}
				type ChangedNumber {
					theNumber: Int!
				}
			`, &resolver{}),
			Query: `
				mutation {
					changeTheNumber(newNumber: 1) {
						theNumber
					}
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{{Message: "no mutations are offered by the schema"}},
		},
		{
			// Explicit schema without mutation field
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}
				type Query {
					hello: String!
				}
				type Mutation {
					changeTheNumber(newNumber: Int!): ChangedNumber!
				}
				type ChangedNumber {
					theNumber: Int!
				}
			`, &resolver{}),
			Query: `
				mutation {
					changeTheNumber(newNumber: 1) {
						theNumber
					}
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{{Message: "no mutations are offered by the schema"}},
		},
	})
}

func TestBasic(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					hero {
						id
						name
						friends {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"hero": {
						"id": "2001",
						"name": "R2-D2",
						"friends": [
							{
								"name": "Luke Skywalker"
							},
							{
								"name": "Han Solo"
							},
							{
								"name": "Leia Organa"
							}
						]
					}
				}
			`,
		},
	})
}

type testEmbeddedStructResolver struct{}

func (*testEmbeddedStructResolver) Course() courseResolver {
	return courseResolver{
		CourseMeta: CourseMeta{
			Name:       "Biology",
			Timestamps: Timestamps{CreatedAt: "yesterday", UpdatedAt: "today"},
		},
		Instructor: Instructor{Name: "Socrates"},
	}
}

type courseResolver struct {
	CourseMeta
	Instructor Instructor
}

type CourseMeta struct {
	Name string
	Timestamps
}

type Instructor struct {
	Name string
}

type Timestamps struct {
	CreatedAt string
	UpdatedAt string
}

func TestEmbeddedStruct(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					course: Course!
				}

				type Course {
					name: String!
					createdAt: String!
					updatedAt: String!
					instructor: Instructor!
				}

				type Instructor {
					name: String!
				}
			`, &testEmbeddedStructResolver{}, graphql.UseFieldResolvers()),
			Query: `
				{
					course{
						name
						createdAt
						updatedAt
						instructor {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"course": {
						"name": "Biology",
						"createdAt": "yesterday",
						"updatedAt": "today",
						"instructor": {
							"name":"Socrates"
						}
					}
				}
			`,
		},
	})
}

type testNilInterfaceResolver struct{}

func (r *testNilInterfaceResolver) A() interface{ Z() int32 } {
	return nil
}

func (r *testNilInterfaceResolver) B() (interface{ Z() int32 }, error) {
	return nil, errors.New("x")
}

func (r *testNilInterfaceResolver) C() (interface{ Z() int32 }, error) {
	return nil, nil
}

func TestNilInterface(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					a: T
					b: T
					c: T
				}

				type T {
					z: Int!
				}
			`, &testNilInterfaceResolver{}),
			Query: `
				{
					a { z }
					b { z }
					c { z }
				}
			`,
			ExpectedResult: `
				{
					"a": null,
					"b": null,
					"c": null
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       "x",
					Path:          []interface{}{"b"},
					ResolverError: errors.New("x"),
				},
			},
		},
	})
}

func TestErrorPropagationInLists(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					findDroids: [Droid!]!
				}
				type Droid {
					name: String!
				}
			`, &findDroidsResolver{}),
			Query: `
				{
					findDroids {
						name
					}
				}
			`,
			ExpectedResult: `
				null
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       droidNotFoundError.Error(),
					Path:          []interface{}{"findDroids", 1, "name"},
					ResolverError: droidNotFoundError,
					Extensions:    map[string]interface{}{"code": droidNotFoundError.Code, "message": droidNotFoundError.Message},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					findDroids: [Droid]!
				}
				type Droid {
					name: String!
				}
			`, &findDroidsResolver{}),
			Query: `
				{
					findDroids {
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"findDroids": [
						{
							"name": "R2-D2"
						},
						null,
						{
							"name": "C-3PO"
						}
					]
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       droidNotFoundError.Error(),
					Path:          []interface{}{"findDroids", 1, "name"},
					ResolverError: droidNotFoundError,
					Extensions:    map[string]interface{}{"code": droidNotFoundError.Code, "message": droidNotFoundError.Message},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					findNilDroids: [Droid!]
				}
				type Droid {
					name: String!
				}
			`, &findDroidsResolver{}),
			Query: `
				{
					findNilDroids {
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"findNilDroids": null
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: `graphql: got nil for non-null "Droid"`,
					Path:    []interface{}{"findNilDroids", 1},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					findNilDroids: [Droid]
				}
				type Droid {
					name: String!
				}
			`, &findDroidsResolver{}),
			Query: `
				{
					findNilDroids {
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"findNilDroids": [
						{
							"name": "R2-D2"
						},
						null,
						{
							"name": "C-3PO"
						}
					]
				}
			`,
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					findDroids: [Droid]!
				}
				type Droid {
					quotes: [String!]!
				}
			`, &findDroidsResolver{}),
			Query: `
				{
					findDroids {
						quotes
					}
				}
			`,
			ExpectedResult: `
				{
					"findDroids": [
						null,
						{
							"quotes": []
						},
						{
							"quotes": [
								"We're doomed!",
								"R2-D2, where are you?"
							]
						}
					]
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       errQuote.Error(),
					ResolverError: errQuote,
					Path:          []interface{}{"findDroids", 0, "quotes"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					findNilDroids: [Droid!]
				}
				type Droid {
					name: String!
					quotes: [String!]!
				}
			`, &findDroidsResolver{}),
			Query: `
				{
					findNilDroids {
						name
						quotes
					}
				}
			`,
			ExpectedResult: `
				{
					"findNilDroids": null
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       errQuote.Error(),
					ResolverError: errQuote,
					Path:          []interface{}{"findNilDroids", 0, "quotes"},
				},
				{
					Message: `graphql: got nil for non-null "Droid"`,
					Path:    []interface{}{"findNilDroids", 1},
				},
			},
		},
	})
}

func TestErrorWithExtensions(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					FindDroid: Droid!
					FindHuman: String
				}
				type Droid {
					Name: String!
				}
			`, &findDroidOrHumanResolver{}),
			Query: `
				{
					FindDroid {
						Name
					}
					FindHuman
				}
			`,
			ExpectedResult: `
				null
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       droidNotFoundError.Error(),
					Path:          []interface{}{"FindDroid"},
					ResolverError: droidNotFoundError,
					Extensions:    map[string]interface{}{"code": droidNotFoundError.Code, "message": droidNotFoundError.Message},
				},
			},
		},
	})
}

func TestErrorWithNoExtensions(t *testing.T) {
	t.Parallel()

	err := errors.New("I find your lack of faith disturbing")

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					DismissVader: String!
				}
			`, &discussPlanResolver{}),
			Query: `
				{
					DismissVader
				}
			`,
			ExpectedResult: `
				null
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       err.Error(),
					Path:          []interface{}{"DismissVader"},
					ResolverError: err,
					Extensions:    nil,
				},
			},
		},
	})
}

func TestArguments(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					human(id: "1000") {
						name
						height
					}
				}
			`,
			ExpectedResult: `
				{
					"human": {
						"name": "Luke Skywalker",
						"height": 1.72
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					human(id: "1000") {
						name
						height(unit: FOOT)
					}
				}
			`,
			ExpectedResult: `
				{
					"human": {
						"name": "Luke Skywalker",
						"height": 5.6430448
					}
				}
			`,
		},
	})
}

func TestAliases(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					empireHero: hero(episode: EMPIRE) {
						name
					}
					jediHero: hero(episode: JEDI) {
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"empireHero": {
						"name": "Luke Skywalker"
					},
					"jediHero": {
						"name": "R2-D2"
					}
				}
			`,
		},
	})
}

func TestFragments(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					leftComparison: hero(episode: EMPIRE) {
						...comparisonFields
						...height
					}
					rightComparison: hero(episode: JEDI) {
						...comparisonFields
						...height
					}
				}

				fragment comparisonFields on Character {
					name
					appearsIn
					friends {
						name
					}
				}

				fragment height on Human {
					height
				}
			`,
			ExpectedResult: `
				{
					"leftComparison": {
						"name": "Luke Skywalker",
						"appearsIn": [
							"NEWHOPE",
							"EMPIRE",
							"JEDI"
						],
						"friends": [
							{
								"name": "Han Solo"
							},
							{
								"name": "Leia Organa"
							},
							{
								"name": "C-3PO"
							},
							{
								"name": "R2-D2"
							}
						],
						"height": 1.72
					},
					"rightComparison": {
						"name": "R2-D2",
						"appearsIn": [
							"NEWHOPE",
							"EMPIRE",
							"JEDI"
						],
						"friends": [
							{
								"name": "Luke Skywalker"
							},
							{
								"name": "Han Solo"
							},
							{
								"name": "Leia Organa"
							}
						]
					}
				}
			`,
		},
		{
			Schema: starwarsSchema,
			Query: `
				query {
					human(id: "1000") {
						id
						mass
						...characterInfo
					}
				}
				fragment characterInfo on Character {
					name
					...on Droid {
						primaryFunction
					}
					...on Human {
						height
					}
				}
			`,
			ExpectedResult: `
				{
					"human": {
						"id": "1000",
						"mass": 77,
						"name": "Luke Skywalker",
						"height": 1.72
					}
				}
			`,
		},
	})
}

func TestVariables(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				query HeroNameAndFriends($episode: Episode) {
					hero(episode: $episode) {
						name
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode": "JEDI",
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2"
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				query HeroNameAndFriends($episode: Episode) {
					hero(episode: $episode) {
						name
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode": "EMPIRE",
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "Luke Skywalker"
					}
				}
			`,
		},

		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					echo(value: String): String
				}
			`, &echoResolver{}),
			Query: `
				query Echo($value:String = "default"){
					echo(value:$value)
				}
			`,
			ExpectedResult: `
				{
					"echo": "default"
				}
			`,
		},
	})
}

func TestSkipDirective(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				query Hero($episode: Episode, $withoutFriends: Boolean!) {
					hero(episode: $episode) {
						name
						friends @skip(if: $withoutFriends) {
							name
						}
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode":        "JEDI",
				"withoutFriends": true,
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2"
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				query Hero($episode: Episode, $withoutFriends: Boolean!) {
					hero(episode: $episode) {
						name
						friends @skip(if: $withoutFriends) {
							name
						}
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode":        "JEDI",
				"withoutFriends": false,
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2",
						"friends": [
							{
								"name": "Luke Skywalker"
							},
							{
								"name": "Han Solo"
							},
							{
								"name": "Leia Organa"
							}
						]
					}
				}
			`,
		},
	})
}

func TestIncludeDirective(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				query Hero($episode: Episode, $withFriends: Boolean!) {
					hero(episode: $episode) {
						name
						...friendsFragment @include(if: $withFriends)
					}
				}

				fragment friendsFragment on Character {
					friends {
						name
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode":     "JEDI",
				"withFriends": false,
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2"
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				query Hero($episode: Episode, $withFriends: Boolean!) {
					hero(episode: $episode) {
						name
						...friendsFragment @include(if: $withFriends)
					}
				}

				fragment friendsFragment on Character {
					friends {
						name
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode":     "JEDI",
				"withFriends": true,
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2",
						"friends": [
							{
								"name": "Luke Skywalker"
							},
							{
								"name": "Han Solo"
							},
							{
								"name": "Leia Organa"
							}
						]
					}
				}
			`,
		},
	})
}

type testDeprecatedDirectiveResolver struct{}

func (r *testDeprecatedDirectiveResolver) A() int32 {
	return 0
}

func (r *testDeprecatedDirectiveResolver) B() int32 {
	return 0
}

func (r *testDeprecatedDirectiveResolver) C() int32 {
	return 0
}

func TestDeprecatedDirective(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					a: Int!
					b: Int! @deprecated
					c: Int! @deprecated(reason: "We don't like it")
				}
			`, &testDeprecatedDirectiveResolver{}),
			Query: `
				{
					__type(name: "Query") {
						fields {
							name
						}
						allFields: fields(includeDeprecated: true) {
							name
							isDeprecated
							deprecationReason
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"__type": {
						"fields": [
							{ "name": "a" }
						],
						"allFields": [
							{ "name": "a", "isDeprecated": false, "deprecationReason": null },
							{ "name": "b", "isDeprecated": true, "deprecationReason": "No longer supported" },
							{ "name": "c", "isDeprecated": true, "deprecationReason": "We don't like it" }
						]
					}
				}
			`,
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
				}

				enum Test {
					A
					B @deprecated
					C @deprecated(reason: "We don't like it")
				}
			`, &testDeprecatedDirectiveResolver{}),
			Query: `
				{
					__type(name: "Test") {
						enumValues {
							name
						}
						allEnumValues: enumValues(includeDeprecated: true) {
							name
							isDeprecated
							deprecationReason
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"__type": {
						"enumValues": [
							{ "name": "A" }
						],
						"allEnumValues": [
							{ "name": "A", "isDeprecated": false, "deprecationReason": null },
							{ "name": "B", "isDeprecated": true, "deprecationReason": "No longer supported" },
							{ "name": "C", "isDeprecated": true, "deprecationReason": "We don't like it" }
						]
					}
				}
			`,
		},
	})
}

func TestSpecifiedByDirective(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
			schema {
				query: Query
			}
			type Query {
			}
			scalar UUID @specifiedBy(
				url: "https://tools.ietf.org/html/rfc4122"
			)
			`, &struct{}{}),
			Query: `
				query {
					__type(name: "UUID") {
						name
						specifiedByURL
					}
				}
			`,
			Variables: map[string]interface{}{},
			ExpectedResult: `
				{
					"__type": {
						"name": "UUID",
						"specifiedByURL": "https://tools.ietf.org/html/rfc4122"
					}
				}
			`,
		},
	})
}

type testBadEnumResolver struct{}

func (r *testBadEnumResolver) Hero() *testBadEnumCharacterResolver {
	return &testBadEnumCharacterResolver{}
}

type testBadEnumCharacterResolver struct{}

func (r *testBadEnumCharacterResolver) Name() string {
	return "Spock"
}

func (r *testBadEnumCharacterResolver) AppearsIn() []string {
	return []string{"STAR_TREK"}
}

func TestUnknownType(t *testing.T) {
	gqltesting.RunTest(t, &gqltesting.Test{
		Schema: starwarsSchema,
		Query: `
			query TypeInfo {
				__type(name: "unknown-type") {
					name
				}
			}
		`,
		ExpectedResult: `
			{
				"__type": null
			}
		`,
	})
}

func TestEnums(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		// Valid input enum supplied in query text
		{
			Schema: starwarsSchema,
			Query: `
				query HeroForEpisode {
					hero(episode: EMPIRE) {
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"hero": {
						"name": "Luke Skywalker"
					}
				}
			`,
		},
		// Invalid input enum supplied in query text
		{
			Schema: starwarsSchema,
			Query: `
				query HeroForEpisode {
					hero(episode: WRATH_OF_KHAN) {
						name
					}
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:   "Argument \"episode\" has invalid value WRATH_OF_KHAN.\nExpected type \"Episode\", found WRATH_OF_KHAN.",
					Locations: []gqlerrors.Location{{Column: 20, Line: 3}},
					Rule:      "ArgumentsOfCorrectType",
				},
			},
		},
		// Valid input enum supplied in variables
		{
			Schema: starwarsSchema,
			Query: `
				query HeroForEpisode($episode: Episode!) {
					hero(episode: $episode) {
						name
					}
				}
			`,
			Variables: map[string]interface{}{"episode": "JEDI"},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2"
					}
				}
			`,
		},
		// Invalid input enum supplied in variables
		{
			Schema: starwarsSchema,
			Query: `
				query HeroForEpisode($episode: Episode!) {
					hero(episode: $episode) {
						name
					}
				}
			`,
			Variables: map[string]interface{}{"episode": "FINAL_FRONTIER"},
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:   "Variable \"episode\" has invalid value FINAL_FRONTIER.\nExpected type \"Episode\", found FINAL_FRONTIER.",
					Locations: []gqlerrors.Location{{Column: 26, Line: 2}},
					Rule:      "VariablesOfCorrectType",
				},
			},
		},
		// Valid enum value in response
		{
			Schema: starwarsSchema,
			Query: `
				query Hero {
					hero {
						name
						appearsIn
					}
				}
			`,
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2",
						"appearsIn": ["NEWHOPE","EMPIRE","JEDI"]
					}
				}
			`,
		},
		// Invalid enum value in response
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					hero: Character
				}

				enum Episode {
					NEWHOPE
					EMPIRE
					JEDI
				}

				type Character {
					name: String!
					appearsIn: [Episode!]!
				}
			`, &testBadEnumResolver{}),
			Query: `
				query Hero {
					hero {
						name
						appearsIn
					}
				}
			`,
			ExpectedResult: `{
				"hero": null
			}`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "Invalid value STAR_TREK.\nExpected type Episode, found STAR_TREK.",
					Path:    []interface{}{"hero", "appearsIn", 0},
				},
			},
		},
	})
}

type testDeprecatedArgsResolver struct{}

func (r *testDeprecatedArgsResolver) A(args struct{ B *string }) int32 {
	return 0
}

func TestDeprecatedArgs(t *testing.T) {
	graphql.MustParseSchema(`
		schema {
			query: Query
		}
		type Query {
			a(b: String @deprecated): Int!
		}
	`, &testDeprecatedArgsResolver{})
}

func TestInlineFragments(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				query HeroForEpisode($episode: Episode!) {
					hero(episode: $episode) {
						name
						... on Droid {
							primaryFunction
						}
						... on Human {
							height
						}
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode": "JEDI",
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2",
						"primaryFunction": "Astromech"
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				query HeroForEpisode($episode: Episode!) {
					hero(episode: $episode) {
						name
						... on Droid {
							primaryFunction
						}
						... on Human {
							height
						}
					}
				}
			`,
			Variables: map[string]interface{}{
				"episode": "EMPIRE",
			},
			ExpectedResult: `
				{
					"hero": {
						"name": "Luke Skywalker",
						"height": 1.72
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				query CharacterSearch {
					search(text: "C-3PO") {
						... on Character {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"search": [
						{
							"name": "C-3PO"
						}
					]
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				query CharacterSearch {
					hero {
						... on Character {
							... on Human {
								name
							}
							... on Droid {
								name
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2"
					}
				}
			`,
		},

		{
			Schema: graphql.MustParseSchema(social.Schema, &social.Resolver{}, graphql.UseFieldResolvers()),
			Query: `
				query {
					admin(id: "0x01") {
						... on User {
							email
						}
						... on Person {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"admin": {
						"email": "Albus@hogwarts.com",
						"name": "Albus Dumbledore"
					}
				}
			`,
		},
	})
}

func TestTypeName(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					search(text: "an") {
						__typename
						... on Human {
							name
						}
						... on Droid {
							name
						}
						... on Starship {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"search": [
						{
							"__typename": "Human",
							"name": "Han Solo"
						},
						{
							"__typename": "Human",
							"name": "Leia Organa"
						},
						{
							"__typename": "Starship",
							"name": "TIE Advanced x1"
						}
					]
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					human(id: "1000") {
						__typename
						name
					}
				}
			`,
			ExpectedResult: `
				{
					"human": {
						"__typename": "Human",
						"name": "Luke Skywalker"
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					hero {
						__typename
						name
						... on Character {
							...Droid
							name
							__typename
						}
					}
				}

				fragment Droid on Droid {
					name
					__typename
				}
			`,
			RawResponse:    true,
			ExpectedResult: `{"hero":{"__typename":"Droid","name":"R2-D2"}}`,
		},
	})
}

func TestConnections(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					hero {
						name
						friendsConnection {
							totalCount
							pageInfo {
								startCursor
								endCursor
								hasNextPage
							}
							edges {
								cursor
								node {
									name
								}
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2",
						"friendsConnection": {
							"totalCount": 3,
							"pageInfo": {
								"startCursor": "Y3Vyc29yMQ==",
								"endCursor": "Y3Vyc29yMw==",
								"hasNextPage": false
							},
							"edges": [
								{
									"cursor": "Y3Vyc29yMQ==",
									"node": {
										"name": "Luke Skywalker"
									}
								},
								{
									"cursor": "Y3Vyc29yMg==",
									"node": {
										"name": "Han Solo"
									}
								},
								{
									"cursor": "Y3Vyc29yMw==",
									"node": {
										"name": "Leia Organa"
									}
								}
							]
						}
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					hero {
						name
						friendsConnection(first: 1, after: "Y3Vyc29yMQ==") {
							totalCount
							pageInfo {
								startCursor
								endCursor
								hasNextPage
							}
							edges {
								cursor
								node {
									name
								}
							}
						}
					},
					moreFriends: hero {
						name
						friendsConnection(first: 1, after: "Y3Vyc29yMg==") {
							totalCount
							pageInfo {
								startCursor
								endCursor
								hasNextPage
							}
							edges {
								cursor
								node {
									name
								}
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"hero": {
						"name": "R2-D2",
						"friendsConnection": {
							"totalCount": 3,
							"pageInfo": {
								"startCursor": "Y3Vyc29yMg==",
								"endCursor": "Y3Vyc29yMg==",
								"hasNextPage": true
							},
							"edges": [
								{
									"cursor": "Y3Vyc29yMg==",
									"node": {
										"name": "Han Solo"
									}
								}
							]
						}
					},
					"moreFriends": {
						"name": "R2-D2",
						"friendsConnection": {
							"totalCount": 3,
							"pageInfo": {
								"startCursor": "Y3Vyc29yMw==",
								"endCursor": "Y3Vyc29yMw==",
								"hasNextPage": false
							},
							"edges": [
							{
								"cursor": "Y3Vyc29yMw==",
								"node": {
									"name": "Leia Organa"
								}
							}
							]
						}
					}
				}
			`,
		},
	})
}

func TestMutation(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					reviews(episode: JEDI) {
						stars
						commentary
					}
				}
			`,
			ExpectedResult: `
				{
					"reviews": []
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				mutation CreateReviewForEpisode($ep: Episode!, $review: ReviewInput!) {
					createReview(episode: $ep, review: $review) {
						stars
						commentary
					}
				}
			`,
			Variables: map[string]interface{}{
				"ep": "JEDI",
				"review": map[string]interface{}{
					"stars":      5,
					"commentary": "This is a great movie!",
				},
			},
			ExpectedResult: `
				{
					"createReview": {
						"stars": 5,
						"commentary": "This is a great movie!"
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				mutation CreateReviewForEpisode($ep: Episode!, $review: ReviewInput!) {
					createReview(episode: $ep, review: $review) {
						stars
						commentary
					}
				}
			`,
			Variables: map[string]interface{}{
				"ep": "EMPIRE",
				"review": map[string]interface{}{
					"stars": float64(4),
				},
			},
			ExpectedResult: `
				{
					"createReview": {
						"stars": 4,
						"commentary": null
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					reviews(episode: JEDI) {
						stars
						commentary
					}
				}
			`,
			ExpectedResult: `
				{
					"reviews": [{
						"stars": 5,
						"commentary": "This is a great movie!"
					}]
				}
			`,
		},
	})
}

func TestIntrospection(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					__schema {
						types {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"__schema": {
						"types": [
							{ "name": "Boolean" },
							{ "name": "Character" },
							{ "name": "Droid" },
							{ "name": "Episode" },
							{ "name": "Float" },
							{ "name": "FriendsConnection" },
							{ "name": "FriendsEdge" },
							{ "name": "Human" },
							{ "name": "ID" },
							{ "name": "Int" },
							{ "name": "LengthUnit" },
							{ "name": "Mutation" },
							{ "name": "PageInfo" },
							{ "name": "Query" },
							{ "name": "Review" },
							{ "name": "ReviewInput" },
							{ "name": "SearchResult" },
							{ "name": "Starship" },
							{ "name": "String" },
							{ "name": "__Directive" },
							{ "name": "__DirectiveLocation" },
							{ "name": "__EnumValue" },
							{ "name": "__Field" },
							{ "name": "__InputValue" },
							{ "name": "__Schema" },
							{ "name": "__Type" },
							{ "name": "__TypeKind" }
						]
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					__schema {
						queryType {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"__schema": {
						"queryType": {
							"name": "Query"
						}
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					a: __type(name: "Droid") {
						name
						kind
						interfaces {
							name
						}
						possibleTypes {
							name
						}
					},
					b: __type(name: "Character") {
						name
						kind
						interfaces {
							name
						}
						possibleTypes {
							name
						}
					}
					c: __type(name: "SearchResult") {
						name
						kind
						interfaces {
							name
						}
						possibleTypes {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"a": {
						"name": "Droid",
						"kind": "OBJECT",
						"interfaces": [
							{
								"name": "Character"
							}
						],
						"possibleTypes": null
					},
					"b": {
						"name": "Character",
						"kind": "INTERFACE",
						"interfaces": null,
						"possibleTypes": [
							{
								"name": "Human"
							},
							{
								"name": "Droid"
							}
						]
					},
					"c": {
						"name": "SearchResult",
						"kind": "UNION",
						"interfaces": null,
						"possibleTypes": [
							{
								"name": "Human"
							},
							{
								"name": "Droid"
							},
							{
								"name": "Starship"
							}
						]
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					__type(name: "Droid") {
						name
						fields {
							name
							args {
								name
								type {
									name
								}
								defaultValue
							}
							type {
								name
								kind
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"__type": {
						"name": "Droid",
						"fields": [
							{
								"name": "id",
								"args": [],
								"type": {
									"name": null,
									"kind": "NON_NULL"
								}
							},
							{
								"name": "name",
								"args": [],
								"type": {
									"name": null,
									"kind": "NON_NULL"
								}
							},
							{
								"name": "friends",
								"args": [],
								"type": {
									"name": null,
									"kind": "LIST"
								}
							},
							{
								"name": "friendsConnection",
								"args": [
									{
										"name": "first",
										"type": {
											"name": "Int"
										},
										"defaultValue": null
									},
									{
										"name": "after",
										"type": {
											"name": "ID"
										},
										"defaultValue": null
									}
								],
								"type": {
									"name": null,
									"kind": "NON_NULL"
								}
							},
							{
								"name": "appearsIn",
								"args": [],
								"type": {
									"name": null,
									"kind": "NON_NULL"
								}
							},
							{
								"name": "primaryFunction",
								"args": [],
								"type": {
									"name": "String",
									"kind": "SCALAR"
								}
							}
						]
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					__type(name: "Episode") {
						enumValues {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"__type": {
						"enumValues": [
							{
								"name": "NEWHOPE"
							},
							{
								"name": "EMPIRE"
							},
							{
								"name": "JEDI"
							}
						]
					}
				}
			`,
		},

		{
			Schema: starwarsSchema,
			Query: `
				{
					__schema {
						directives {
							name
							description
							locations
							args {
								name
								description
								type {
									kind
									ofType {
										kind
										name
									}
								}
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
						"__schema": {
							"directives": [
								{
									"name": "deprecated",
									"description": "Marks an element of a GraphQL schema as no longer supported.",
									"locations": [
										"FIELD_DEFINITION",
										"ENUM_VALUE",
										"ARGUMENT_DEFINITION"
									],
									"args": [
										{
											"name": "reason",
											"description": "Explains why this element was deprecated, usually also including a suggestion\nfor how to access supported similar data. Formatted in\n[Markdown](https://daringfireball.net/projects/markdown/).",
											"type": {
												"kind": "SCALAR",
												"ofType": null
											}
										}
									]
								},
								{
									"name": "include",
									"description": "Directs the executor to include this field or fragment only when the ` + "`" + `if` + "`" + ` argument is true.",
									"locations": [
										"FIELD",
										"FRAGMENT_SPREAD",
										"INLINE_FRAGMENT"
									],
									"args": [
										{
											"name": "if",
											"description": "Included when true.",
											"type": {
												"kind": "NON_NULL",
												"ofType": {
													"kind": "SCALAR",
													"name": "Boolean"
												}
											}
										}
									]
								},
								{
									"name": "skip",
									"description": "Directs the executor to skip this field or fragment when the ` + "`" + `if` + "`" + ` argument is true.",
									"locations": [
										"FIELD",
										"FRAGMENT_SPREAD",
										"INLINE_FRAGMENT"
									],
									"args": [
										{
											"name": "if",
											"description": "Skipped when true.",
											"type": {
												"kind": "NON_NULL",
												"ofType": {
													"kind": "SCALAR",
													"name": "Boolean"
												}
											}
										}
									]
								},
								{
									"name": "specifiedBy",
									"description": "Provides a scalar specification URL for specifying the behavior of custom scalar types.",
									"locations": [
										"SCALAR"
									],
									"args": [
										{
											"name": "url",
											"description": "The URL should point to a human-readable specification of the data format, serialization, and coercion rules.",
											"type": {
												"kind": "NON_NULL",
												"ofType": {
													"kind": "SCALAR",
													"name": "String"
												}
											}
										}
									]
								}
							]
						}
					}
			`,
		},
	})
}

var starwarsSchemaNoIntrospection = graphql.MustParseSchema(starwars.Schema, &starwars.Resolver{}, []graphql.SchemaOpt{graphql.DisableIntrospection()}...)

func TestIntrospectionDisableIntrospection(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					__schema {
						types {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
				}
			`,
		},

		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					__schema {
						queryType {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
				}
			`,
		},

		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					a: __type(name: "Droid") {
						name
						kind
						interfaces {
							name
						}
						possibleTypes {
							name
						}
					},
					b: __type(name: "Character") {
						name
						kind
						interfaces {
							name
						}
						possibleTypes {
							name
						}
					}
					c: __type(name: "SearchResult") {
						name
						kind
						interfaces {
							name
						}
						possibleTypes {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
				}
			`,
		},

		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					__type(name: "Droid") {
						name
						fields {
							name
							args {
								name
								type {
									name
								}
								defaultValue
							}
							type {
								name
								kind
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
				}
			`,
		},

		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					__type(name: "Episode") {
						enumValues {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
				}
			`,
		},

		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					__schema {
						directives {
							name
							description
							locations
							args {
								name
								description
								type {
									kind
									ofType {
										kind
										name
									}
								}
							}
						}
					}
				}
			`,
			ExpectedResult: `
				{
				}
			`,
		},

		{
			Schema: starwarsSchemaNoIntrospection,
			Query: `
				{
					search(text: "an") {
						__typename
						... on Human {
							name
						}
						... on Droid {
							name
						}
						... on Starship {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"search": [
						{
							"__typename": "Human",
							"name": "Han Solo"
						},
						{
							"__typename": "Human",
							"name": "Leia Organa"
						},
						{
							"__typename": "Starship",
							"name": "TIE Advanced x1"
						}
					]
				}
			`,
		},
	})
}

func TestMutationOrder(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
					mutation: Mutation
				}

				type Query {
					theNumber: Int!
				}

				type Mutation {
					changeTheNumber(newNumber: Int!): Query
				}
			`, &theNumberResolver{}),
			Query: `
				mutation {
					first: changeTheNumber(newNumber: 1) {
						theNumber
					}
					second: changeTheNumber(newNumber: 3) {
						theNumber
					}
					third: changeTheNumber(newNumber: 2) {
						theNumber
					}
				}
			`,
			ExpectedResult: `
				{
					"first": {
						"theNumber": 1
					},
					"second": {
						"theNumber": 3
					},
					"third": {
						"theNumber": 2
					}
				}
			`,
		},
	})
}

func TestTime(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					addHour(time: Time = "2001-02-03T04:05:06Z"): Time!
				}

				scalar Time
			`, &timeResolver{}),
			Query: `
				query($t: Time!) {
					a: addHour(time: $t)
					b: addHour
				}
			`,
			Variables: map[string]interface{}{
				"t": time.Date(2000, 2, 3, 4, 5, 6, 0, time.UTC),
			},
			ExpectedResult: `
				{
					"a": "2000-02-03T05:05:06Z",
					"b": "2001-02-03T05:05:06Z"
				}
			`,
		},
	})
}

type resolverWithUnexportedMethod struct{}

//nolint:unused // Method is intentionally left unused to test unexported methods.
func (r *resolverWithUnexportedMethod) changeTheNumber(args struct{ NewNumber int32 }) int32 {
	return args.NewNumber
}

func TestUnexportedMethod(t *testing.T) {
	t.Parallel()

	_, err := graphql.ParseSchema(`
		schema {
			mutation: Mutation
		}

		type Mutation {
			changeTheNumber(newNumber: Int!): Int!
		}
	`, &resolverWithUnexportedMethod{})
	if err == nil {
		t.Error("error expected")
	}
}

type resolverWithUnexportedField struct{}

func (r *resolverWithUnexportedField) ChangeTheNumber(args struct{ newNumber int32 }) int32 {
	return args.newNumber
}

func TestUnexportedField(t *testing.T) {
	t.Parallel()

	_, err := graphql.ParseSchema(`
		schema {
			mutation: Mutation
		}

		type Mutation {
			changeTheNumber(newNumber: Int!): Int!
		}
	`, &resolverWithUnexportedField{})
	if err == nil {
		t.Error("error expected")
	}
}

type StringEnum string

const (
	EnumOption1 StringEnum = "Option1"
	EnumOption2 StringEnum = "Option2"
)

type IntEnum int

const (
	IntEnum0 IntEnum = iota
	IntEnum1
)

func (e IntEnum) String() string {
	switch int(e) {
	case 0:
		return "Int0"
	case 1:
		return "Int1"
	default:
		return "IntN"
	}
}

func (IntEnum) ImplementsGraphQLType(name string) bool {
	return name == "IntEnum"
}

func (e *IntEnum) UnmarshalGraphQL(input interface{}) error {
	if str, ok := input.(string); ok {
		switch str {
		case "Int0":
			*e = IntEnum(0)
		case "Int1":
			*e = IntEnum(1)
		default:
			*e = IntEnum(-1)
		}
		return nil
	}
	return fmt.Errorf("wrong type for IntEnum: %T", input)
}

type inputResolver struct{}

func (r *inputResolver) Int(args struct{ Value int32 }) int32 {
	return args.Value
}

func (r *inputResolver) Float(args struct{ Value float64 }) float64 {
	return args.Value
}

func (r *inputResolver) String(args struct{ Value string }) string {
	return args.Value
}

func (r *inputResolver) Boolean(args struct{ Value bool }) bool {
	return args.Value
}

func (r *inputResolver) Nullable(args struct{ Value *int32 }) *int32 {
	return args.Value
}

func (r *inputResolver) List(args struct{ Value []*struct{ V int32 } }) []int32 {
	l := make([]int32, len(args.Value))
	for i, entry := range args.Value {
		l[i] = entry.V
	}
	return l
}

func (r *inputResolver) NullableList(args struct{ Value *[]*struct{ V int32 } }) *[]*int32 {
	if args.Value == nil {
		return nil
	}
	l := make([]*int32, len(*args.Value))
	for i, entry := range *args.Value {
		if entry != nil {
			l[i] = &entry.V
		}
	}
	return &l
}

func (r *inputResolver) StringEnumValue(args struct{ Value string }) string {
	return args.Value
}

func (r *inputResolver) NullableStringEnumValue(args struct{ Value *string }) *string {
	return args.Value
}

func (r *inputResolver) StringEnum(args struct{ Value StringEnum }) StringEnum {
	return args.Value
}

func (r *inputResolver) NullableStringEnum(args struct{ Value *StringEnum }) *StringEnum {
	return args.Value
}

func (r *inputResolver) IntEnumValue(args struct{ Value string }) string {
	return args.Value
}

func (r *inputResolver) NullableIntEnumValue(args struct{ Value *string }) *string {
	return args.Value
}

func (r *inputResolver) IntEnum(args struct{ Value IntEnum }) IntEnum {
	return args.Value
}

func (r *inputResolver) NullableIntEnum(args struct{ Value *IntEnum }) *IntEnum {
	return args.Value
}

type recursive struct {
	Next *recursive
}

func (r *inputResolver) Recursive(args struct{ Value *recursive }) int32 {
	var n int32
	v := args.Value
	for v != nil {
		v = v.Next
		n++
	}
	return n
}

func (r *inputResolver) ID(args struct{ Value graphql.ID }) graphql.ID {
	return args.Value
}

func TestInput(t *testing.T) {
	t.Parallel()

	coercionSchema := graphql.MustParseSchema(`
		schema {
			query: Query
		}

		type Query {
			int(value: Int!): Int!
			float(value: Float!): Float!
			string(value: String!): String!
			boolean(value: Boolean!): Boolean!
			nullable(value: Int): Int
			list(value: [Input!]!): [Int!]!
			nullableList(value: [Input]): [Int]
			stringEnumValue(value: StringEnum!): StringEnum!
			nullableStringEnumValue(value: StringEnum): StringEnum
			stringEnum(value: StringEnum!): StringEnum!
			nullableStringEnum(value: StringEnum): StringEnum
			intEnumValue(value: IntEnum!): IntEnum!
			nullableIntEnumValue(value: IntEnum): IntEnum
			intEnum(value: IntEnum!): IntEnum!
			nullableIntEnum(value: IntEnum): IntEnum
			recursive(value: RecursiveInput): Int!
			id(value: ID!): ID!
		}

		input Input {
			v: Int!
		}

		input RecursiveInput {
			next: RecursiveInput
		}

		enum StringEnum {
			Option1
			Option2
		}

		enum IntEnum {
			Int0
			Int1
		}
	`, &inputResolver{})
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: coercionSchema,
			Query: `
				{
					int(value: 42)
					float1: float(value: 42)
					float2: float(value: 42.5)
					string(value: "foo")
					boolean(value: true)
					nullable1: nullable(value: 42)
					nullable2: nullable(value: null)
					list1: list(value: [{v: 41}, {v: 42}, {v: 43}])
					list2: list(value: {v: 42})
					nullableList1: nullableList(value: [{v: 41}, null, {v: 43}])
					nullableList2: nullableList(value: null)
					stringEnumValue(value: Option1)
					nullableStringEnumValue1: nullableStringEnum(value: Option1)
					nullableStringEnumValue2: nullableStringEnum(value: null)
					stringEnum(value: Option2)
					nullableStringEnum1: nullableStringEnum(value: Option2)
					nullableStringEnum2: nullableStringEnum(value: null)
					intEnumValue(value: Int1)
					nullableIntEnumValue1: nullableIntEnumValue(value: Int1)
					nullableIntEnumValue2: nullableIntEnumValue(value: null)
					intEnum(value: Int1)
					nullableIntEnum1: nullableIntEnum(value: Int1)
					nullableIntEnum2: nullableIntEnum(value: null)
					recursive(value: {next: {next: {}}})
					intID: id(value: 1234)
					strID: id(value: "1234")
				}
			`,
			ExpectedResult: `
				{
					"int": 42,
					"float1": 42,
					"float2": 42.5,
					"string": "foo",
					"boolean": true,
					"nullable1": 42,
					"nullable2": null,
					"list1": [41, 42, 43],
					"list2": [42],
					"nullableList1": [41, null, 43],
					"nullableList2": null,
					"stringEnumValue": "Option1",
					"nullableStringEnumValue1": "Option1",
					"nullableStringEnumValue2": null,
					"stringEnum": "Option2",
					"nullableStringEnum1": "Option2",
					"nullableStringEnum2": null,
					"intEnumValue": "Int1",
					"nullableIntEnumValue1": "Int1",
					"nullableIntEnumValue2": null,
					"intEnum": "Int1",
					"nullableIntEnum1": "Int1",
					"nullableIntEnum2": null,
					"recursive": 3,
					"intID": "1234",
					"strID": "1234"
				}
			`,
		},
	})
}

type inputArgumentsHello struct{}

type inputArgumentsScalarMismatch1 struct{}

type inputArgumentsScalarMismatch2 struct{}

type inputArgumentsObjectMismatch1 struct{}

type inputArgumentsObjectMismatch2 struct{}

type inputArgumentsObjectMismatch3 struct{}

type fieldNameMismatch struct{}

type helloInput struct {
	Name string
}

type helloOutput struct {
	Name string
}

func (*fieldNameMismatch) Hello() helloOutput {
	return helloOutput{}
}

type helloInputMismatch struct {
	World string
}

func (r *inputArgumentsHello) Hello(args struct{ Input *helloInput }) string {
	return "Hello " + args.Input.Name + "!"
}

func (r *inputArgumentsScalarMismatch1) Hello(name string) string {
	return "Hello " + name + "!"
}

func (r *inputArgumentsScalarMismatch2) Hello(args struct{ World string }) string {
	return "Hello " + args.World + "!"
}

func (r *inputArgumentsObjectMismatch1) Hello(in helloInput) string {
	return "Hello " + in.Name + "!"
}

func (r *inputArgumentsObjectMismatch2) Hello(args struct{ Input *helloInputMismatch }) string {
	return "Hello " + args.Input.World + "!"
}

func (r *inputArgumentsObjectMismatch3) Hello(args struct{ Input *struct{ Thing string } }) string {
	return "Hello " + args.Input.Thing + "!"
}

func TestInputArguments_failSchemaParsing(t *testing.T) {
	type args struct {
		Resolver interface{}
		Schema   string
		Opts     []graphql.SchemaOpt
	}
	type want struct {
		Error string
	}
	testTable := map[string]struct {
		Args args
		Want want
	}{
		"Non-input type used with field arguments": {
			Args: args{
				Resolver: &inputArgumentsHello{},
				Schema: `
					schema {
						query: Query
					}
					type Query {
						hello(input: HelloInput): String!
					}
					type HelloInput {
						name: String
					}
				`,
			},
			Want: want{Error: "field \"Input\": type of kind OBJECT can not be used as input\n\tused by (*graphql_test.inputArgumentsHello).Hello"},
		},
		"Missing Args Wrapper for scalar input": {
			Args: args{
				Resolver: &inputArgumentsScalarMismatch1{},
				Schema: `
					schema {
						query: Query
					}
					type Query {
						hello(name: String): String!
					}
					input HelloInput {
						name: String
					}
				`,
			},
			Want: want{Error: "expected struct or pointer to struct, got string (hint: missing `args struct { ... }` wrapper for field arguments?)\n\tused by (*graphql_test.inputArgumentsScalarMismatch1).Hello"},
		},
		"Mismatching field name for scalar input": {
			Args: args{
				Resolver: &inputArgumentsScalarMismatch2{},
				Schema: `
					schema {
						query: Query
					}
					type Query {
						hello(name: String): String!
					}
				`,
			},
			Want: want{Error: "struct { World string } does not define field \"name\" (hint: missing `args struct { ... }` wrapper for field arguments, or missing field on input struct)\n\tused by (*graphql_test.inputArgumentsScalarMismatch2).Hello"},
		},
		"Missing Args Wrapper for Input type": {
			Args: args{
				Resolver: &inputArgumentsObjectMismatch1{},
				Schema: `
					schema {
						query: Query
					}
					type Query {
						hello(input: HelloInput): String!
					}
					input HelloInput {
						name: String
					}
				`,
			},
			Want: want{Error: "graphql_test.helloInput does not define field \"input\" (hint: missing `args struct { ... }` wrapper for field arguments, or missing field on input struct)\n\tused by (*graphql_test.inputArgumentsObjectMismatch1).Hello"},
		},
		"Input struct missing field": {
			Args: args{
				Resolver: &inputArgumentsObjectMismatch2{},
				Schema: `
					schema {
						query: Query
					}
					type Query {
						hello(input: HelloInput): String!
					}
					input HelloInput {
						name: String
					}
				`,
			},
			Want: want{Error: "field \"Input\": *graphql_test.helloInputMismatch does not define field \"name\" (hint: missing `args struct { ... }` wrapper for field arguments, or missing field on input struct)\n\tused by (*graphql_test.inputArgumentsObjectMismatch2).Hello"},
		},
		"Inline Input struct missing field": {
			Args: args{
				Resolver: &inputArgumentsObjectMismatch3{},
				Schema: `
					schema {
						query: Query
					}
					type Query {
						hello(input: HelloInput): String!
					}
					input HelloInput {
						name: String
					}
				`,
			},
			Want: want{Error: "field \"Input\": *struct { Thing string } does not define field \"name\" (hint: missing `args struct { ... }` wrapper for field arguments, or missing field on input struct)\n\tused by (*graphql_test.inputArgumentsObjectMismatch3).Hello"},
		},
		"Struct field name inclusion": {
			Args: args{
				Resolver: &fieldNameMismatch{},
				Opts:     []graphql.SchemaOpt{graphql.UseFieldResolvers()},
				Schema: `
					type Query {
						hello(): HelloOutput!
					}
					type HelloOutput {
						name: Int
					}
				`,
			},
			Want: want{Error: "string is not a pointer\n\tused by (graphql_test.helloOutput).Name\n\tused by (*graphql_test.fieldNameMismatch).Hello"},
		},
	}

	for name, tt := range testTable {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := graphql.ParseSchema(tt.Args.Schema, tt.Args.Resolver, tt.Args.Opts...)
			if err == nil || err.Error() != tt.Want.Error {
				t.Log("Schema parsing error mismatch")
				t.Logf("got: %s", err)
				t.Logf("exp: %s", tt.Want.Error)
				t.Fail()
			}
		})
	}
}

func TestComposedFragments(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: starwarsSchema,
			Query: `
				{
					composed: hero(episode: EMPIRE) {
						name
						...friendsNames
						...friendsIds
					}
				}

				fragment friendsNames on Character {
					name
					friends {
						name
					}
				}

				fragment friendsIds on Character {
					name
					friends {
						id
					}
				}
			`,
			ExpectedResult: `
				{
					"composed": {
						"name": "Luke Skywalker",
						"friends": [
							{
								"id": "1002",
								"name": "Han Solo"
							},
							{
								"id": "1003",
								"name": "Leia Organa"
							},
							{
								"id": "2000",
								"name": "C-3PO"
							},
							{
								"id": "2001",
								"name": "R2-D2"
							}
						]
					}
				}
			`,
		},
	})
}

var (
	errExample = fmt.Errorf("this is an error")

	nilChildErrorString = `graphql: got nil for non-null "Child"`
)

type childResolver struct{}

func (r *childResolver) TriggerError() (string, error) {
	return "This will never be returned to the client", errExample
}

func (r *childResolver) NoError() string {
	return "no error"
}

func (r *childResolver) Child() *childResolver {
	return &childResolver{}
}

func (r *childResolver) NilChild() *childResolver {
	return nil
}

func TestErrorPropagation(t *testing.T) {
	t.Parallel()

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					noError: String!
					triggerError: String!
				}
			`, &childResolver{}),
			Query: `
				{
					noError
					triggerError
				}
			`,
			ExpectedResult: `
				null
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       errExample.Error(),
					ResolverError: errExample,
					Path:          []interface{}{"triggerError"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					noError: String!
					child: Child
				}

				type Child {
					noError: String!
					triggerError: String!
				}
			`, &childResolver{}),
			Query: `
				{
					noError
					child {
						noError
						triggerError
					}
				}
			`,
			ExpectedResult: `
				{
					"noError": "no error",
					"child": null
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       errExample.Error(),
					ResolverError: errExample,
					Path:          []interface{}{"child", "triggerError"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					noError: String!
					child: Child
				}

				type Child {
					noError: String!
					triggerError: String!
					child: Child!
				}
			`, &childResolver{}),
			Query: `
				{
					noError
					child {
						noError
						child {
							noError
							triggerError
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"noError": "no error",
					"child": null
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       errExample.Error(),
					ResolverError: errExample,
					Path:          []interface{}{"child", "child", "triggerError"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					noError: String!
					child: Child
				}

				type Child {
					noError: String!
					triggerError: String!
					child: Child
				}
			`, &childResolver{}),
			Query: `
				{
					noError
					child {
						noError
						child {
							noError
							triggerError
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"noError": "no error",
					"child": {
						"noError": "no error",
						"child": null
					}
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message:       errExample.Error(),
					ResolverError: errExample,
					Path:          []interface{}{"child", "child", "triggerError"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					noError: String!
					child: Child!
				}

				type Child {
					noError: String!
					nilChild: Child!
				}
			`, &childResolver{}),
			Query: `
				{
					noError
					child {
						nilChild {
							noError
						}
					}
				}
			`,
			ExpectedResult: `
				null
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: nilChildErrorString,
					Path:    []interface{}{"child", "nilChild"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					noError: String!
					child: Child
				}

				type Child {
					noError: String!
					nilChild: Child!
				}
			`, &childResolver{}),
			Query: `
				{
					noError
					child {
						noError
						nilChild {
							noError
						}
					}
				}
			`,
			ExpectedResult: `
			{
				"noError": "no error",
				"child": null
			}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: nilChildErrorString,
					Path:    []interface{}{"child", "nilChild"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					child: Child
				}

				type Child {
					triggerError: String!
					child: Child
					nilChild: Child!
				}
			`, &childResolver{}),
			Query: `
				{
					child {
						child {
							triggerError
							child {
								nilChild {
									triggerError
								}
							}
						}
					}
				}
			`,
			ExpectedResult: `
			{
				"child": {
					"child": null
				}
			}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: nilChildErrorString,
					Path:    []interface{}{"child", "child", "child", "nilChild"},
				},
				{
					Message:       errExample.Error(),
					ResolverError: errExample,
					Path:          []interface{}{"child", "child", "triggerError"},
				},
			},
		},
		{
			Schema: graphql.MustParseSchema(`
				schema {
					query: Query
				}

				type Query {
					child: Child
				}

				type Child {
					noError: String!
					child: Child!
					nilChild: Child!
				}
			`, &childResolver{}),
			Query: `
				{
					child {
						child {
							nilChild {
								noError
							}
						}
					}
				}
			`,
			ExpectedResult: `
			{
				"child": null
			}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: nilChildErrorString,
					Path:    []interface{}{"child", "child", "nilChild"},
				},
			},
		},
	})
}

type assertionResolver struct{}

func (r *assertionResolver) ToHuman() (*struct{ Name string }, bool) {
	return &struct{ Name string }{Name: "Luke Skywalker"}, true
}

type assertionQueryResolver struct{}

func (*assertionQueryResolver) Character() *assertionResolver {
	return &assertionResolver{}
}

type badAssertionResolver struct{}

func (r *badAssertionResolver) ToHuman(ctx context.Context) (*struct{ Name string }, bool) {
	return &struct{ Name string }{Name: "Luke Skywalker"}, true
}

type badAssertionQueryResolver struct{}

func (*badAssertionQueryResolver) Character() *badAssertionResolver {
	return &badAssertionResolver{}
}

func TestTypeAssertions(t *testing.T) {
	assertionSchema := `
		schema {
			query: Query
		}

		type Query {
			character: Character!
		}

		type Human {
			name: String!
		}

		union Character = Human
	`
	query := `
		query {
			character {
				... on Human {
					name
				}
			}
		}
	`

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(assertionSchema, &assertionQueryResolver{}, graphql.UseFieldResolvers()),
			Query:  query,
			ExpectedResult: `
				{
					"character": {
						"name": "Luke Skywalker"
					}
				}
			`,
		},
	})
}

func TestPanicTypeAssertionArguments(t *testing.T) {
	panicMessage := `*graphql_test.badAssertionResolver does not resolve "Character": method "ToHuman" should't have any arguments
	used by (*graphql_test.badAssertionQueryResolver).Character`

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected schema parse to panic")
		}

		if r.(error).Error() != panicMessage {
			t.Logf("got:  %s", r)
			t.Logf("want: %s", panicMessage)
			t.Fail()
		}
	}()

	schema := `
		schema {
			query: Query
		}

		type Query {
			character: Character!
		}

		type Human {
			name: String!
		}

		union Character = Human
	`
	graphql.MustParseSchema(schema, &badAssertionQueryResolver{}, graphql.UseFieldResolvers())
}

type ambiguousResolver struct {
	Name string // ambiguous
	University
}

type University struct {
	Name string // ambiguous
}

func TestPanicAmbiguity(t *testing.T) {
	panicMessage := `*graphql_test.ambiguousResolver does not resolve "Query": ambiguous field "name"`

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected schema parse to panic")
		}

		if r.(error).Error() != panicMessage {
			t.Logf("got:  %s", r)
			t.Logf("want: %s", panicMessage)
			t.Fail()
		}
	}()

	schema := `
		schema {
			query: Query
		}

		type Query {
			name: String!
			university: University!
		}

		type University {
			name: String!
		}
	`
	graphql.MustParseSchema(schema, &ambiguousResolver{}, graphql.UseFieldResolvers())
}

func TestSchema_Exec_without_resolver(t *testing.T) {
	t.Parallel()

	type args struct {
		Query  string
		Schema string
	}
	type want struct {
		Panic interface{}
	}
	testTable := []struct {
		Name string
		Args args
		Want want
	}{
		{
			Name: "schema_without_resolver_errors",
			Args: args{
				Query: `
					query {
						hero {
							id
							name
							friends {
								name
							}
						}
					}
				`,
				Schema: starwars.Schema,
			},
			Want: want{Panic: "schema created without resolver, can not exec"},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.Name, func(t *testing.T) {
			s := graphql.MustParseSchema(tt.Args.Schema, nil)

			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected query to panic")
				}
				if r != tt.Want.Panic {
					t.Logf("got:  %s", r)
					t.Logf("want: %s", tt.Want.Panic)
					t.Fail()
				}
			}()
			_ = s.Exec(context.Background(), tt.Args.Query, "", map[string]interface{}{})
		})
	}
}

type subscriptionsInExecResolver struct{}

func (r *subscriptionsInExecResolver) AppUpdated() <-chan string {
	return make(chan string)
}

func TestSubscriptions_In_Exec(t *testing.T) {
	r := &struct {
		*helloResolver
		*subscriptionsInExecResolver
	}{
		helloResolver:               &helloResolver{},
		subscriptionsInExecResolver: &subscriptionsInExecResolver{},
	}
	gqltesting.RunTest(t, &gqltesting.Test{
		Schema: graphql.MustParseSchema(`
			type Query {
				hello: String!
			}
			type Subscription {
				appUpdated : String!
			}
		`, r),
		Query: `
			subscription {
				appUpdated
		  	}
		`,
		ExpectedErrors: []*gqlerrors.QueryError{
			{
				Message: "graphql-ws protocol header is missing",
			},
		},
	})
}

type nilPointerReturnValue struct{}

func (r *nilPointerReturnValue) Value() *string {
	return nil
}

type nilPointerReturnResolver struct{}

func (r *nilPointerReturnResolver) PointerReturn() *nilPointerReturnValue {
	return &nilPointerReturnValue{}
}

func TestPointerReturnForNonNull(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
			type Query {
				pointerReturn: PointerReturnValue
			}

			type PointerReturnValue {
				value: Hello!
			}
			enum Hello {
				WORLD
			}
		`, &nilPointerReturnResolver{}),
			Query: `
				query {
					pointerReturn {
						value
					}
				}
			`,
			ExpectedResult: `
				{
					"pointerReturn": null
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: `graphql: got nil for non-null "Hello"`,
					Path:    []interface{}{"pointerReturn", "value"},
				},
			},
		},
	})
}

type nullableInput struct {
	String graphql.NullString
	Int    graphql.NullInt
	Bool   graphql.NullBool
	Time   graphql.NullTime
	Float  graphql.NullFloat
}

type nullableResult struct {
	String string
	Int    string
	Bool   string
	Time   string
	Float  string
}

type nullableResolver struct{}

func (r *nullableResolver) TestNullables(args struct {
	Input *nullableInput
},
) nullableResult {
	var res nullableResult
	if args.Input.String.Set {
		if args.Input.String.Value == nil {
			res.String = "<nil>"
		} else {
			res.String = *args.Input.String.Value
		}
	}

	if args.Input.Int.Set {
		if args.Input.Int.Value == nil {
			res.Int = "<nil>"
		} else {
			res.Int = fmt.Sprintf("%d", *args.Input.Int.Value)
		}
	}

	if args.Input.Float.Set {
		if args.Input.Float.Value == nil {
			res.Float = "<nil>"
		} else {
			res.Float = fmt.Sprintf("%.2f", *args.Input.Float.Value)
		}
	}

	if args.Input.Bool.Set {
		if args.Input.Bool.Value == nil {
			res.Bool = "<nil>"
		} else {
			res.Bool = fmt.Sprintf("%t", *args.Input.Bool.Value)
		}
	}

	if args.Input.Time.Set {
		if args.Input.Time.Value == nil {
			res.Time = "<nil>"
		} else {
			res.Time = args.Input.Time.Value.Format(time.RFC3339)
		}
	}

	return res
}

func TestNullable(t *testing.T) {
	schema := `
	scalar Time

	input MyInput {
		string: String
		int: Int
		float: Float
		bool: Boolean
		time: Time
	}

	type Result {
		string: String!
		int: String!
		float: String!
		bool: String!
		time: String!
	}

	type Query {
		testNullables(input: MyInput): Result!
	}
	`

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(schema, &nullableResolver{}, graphql.UseFieldResolvers()),
			Query: `
				query {
					testNullables(input: {
						string: "test"
						int: 1234
						float: 42.42
						bool: true
						time: "2021-01-02T15:04:05Z"
					}) {
						string
						int
						float
						bool
						time
					}
				}
			`,
			ExpectedResult: `
				{
					"testNullables": {
						"string": "test",
						"int": "1234",
						"float": "42.42",
						"bool": "true",
						"time": "2021-01-02T15:04:05Z"
					}
				}
			`,
		},
		{
			Schema: graphql.MustParseSchema(schema, &nullableResolver{}, graphql.UseFieldResolvers()),
			Query: `
				query {
					testNullables(input: {
						string: null
						int: null
						float: null
						bool: null
						time: null
					}) {
						string
						int
						float
						bool
						time
					}
				}
			`,
			ExpectedResult: `
				{
					"testNullables": {
						"string": "<nil>",
						"int": "<nil>",
						"float": "<nil>",
						"bool": "<nil>",
						"time": "<nil>"
					}
				}
			`,
		},
		{
			Schema: graphql.MustParseSchema(schema, &nullableResolver{}, graphql.UseFieldResolvers()),
			Query: `
				query {
					testNullables(input: {}) {
						string
						int
						float
						bool
						time
					}
				}
			`,
			ExpectedResult: `
				{
					"testNullables": {
						"string": "",
						"int": "",
						"float": "",
						"bool": "",
						"time": ""
					}
				}
			`,
		},
	})
}

type testTracer struct {
	mu      *sync.Mutex
	fields  []fieldTrace
	queries []queryTrace
}

type fieldTrace struct {
	label     string
	typeName  string
	fieldName string
	isTrivial bool
	args      map[string]interface{}
	err       *gqlerrors.QueryError
}

type queryTrace struct {
	document  string
	opName    string
	variables map[string]interface{}
	varTypes  map[string]*introspection.Type
	errors    []*gqlerrors.QueryError
}

func (t *testTracer) TraceField(ctx context.Context, label, typeName, fieldName string, trivial bool, args map[string]interface{}) (context.Context, func(*gqlerrors.QueryError)) {
	return ctx, func(qe *gqlerrors.QueryError) {
		t.mu.Lock()
		defer t.mu.Unlock()

		ft := fieldTrace{
			label:     label,
			typeName:  typeName,
			fieldName: fieldName,
			isTrivial: trivial,
			args:      args,
			err:       qe,
		}

		t.fields = append(t.fields, ft)
	}
}

func (t *testTracer) TraceQuery(ctx context.Context, document string, opName string, vars map[string]interface{}, varTypes map[string]*introspection.Type) (context.Context, func([]*gqlerrors.QueryError)) {
	return ctx, func(qe []*gqlerrors.QueryError) {
		t.mu.Lock()
		defer t.mu.Unlock()

		qt := queryTrace{
			document:  document,
			opName:    opName,
			variables: vars,
			varTypes:  varTypes,
			errors:    qe,
		}

		t.queries = append(t.queries, qt)
	}
}

var _ tracer.Tracer = (*testTracer)(nil)

func TestTracer(t *testing.T) {
	t.Parallel()

	tt := &testTracer{mu: &sync.Mutex{}}

	schema, err := graphql.ParseSchema(starwars.Schema, &starwars.Resolver{}, graphql.Tracer(tt))
	if err != nil {
		t.Fatalf("graphql.ParseSchema: %s", err)
	}

	ctx := context.Background()
	doc := `
	query TestTracer($id: ID!) {
		HanSolo: human(id: $id) {
			__typename
			name
		}
	}
	`
	opName := "TestTracer"
	variables := map[string]interface{}{
		"id": "1002",
	}

	_ = schema.Exec(ctx, doc, opName, variables)

	tt.mu.Lock()
	defer tt.mu.Unlock()

	if len(tt.queries) != 1 {
		t.Fatalf("expected one query trace, but got %d: %#v", len(tt.queries), tt.queries)
	}

	qt := tt.queries[0]
	if qt.document != doc {
		t.Errorf("mismatched query trace document:\nwant: %q\ngot : %q", doc, qt.document)
	}
	if qt.opName != opName {
		t.Errorf("mismated query trace operationName:\nwant: %q\ngot : %q", opName, qt.opName)
	}

	expectedFieldTraces := []fieldTrace{
		{fieldName: "human", typeName: "Query"},
		{fieldName: "__typename", typeName: "Human"},
		{fieldName: "name", typeName: "Human"},
	}

	checkFieldTraces(t, expectedFieldTraces, tt.fields)
}

func checkFieldTraces(t *testing.T, want, have []fieldTrace) {
	if len(want) != len(have) {
		t.Errorf("mismatched field traces: expected %d but got %d: %#v", len(want), len(have), have)
	}

	type comparison struct {
		want fieldTrace
		have fieldTrace
	}

	m := map[string]comparison{}

	for _, ft := range want {
		m[ft.fieldName] = comparison{want: ft}
	}

	for _, ft := range have {
		c := m[ft.fieldName]
		c.have = ft
		m[ft.fieldName] = c
	}

	for _, c := range m {
		if err := stringsEqual(c.want.fieldName, c.have.fieldName); err != "" {
			t.Error("mismatched field name:", err)
		}
		if err := stringsEqual(c.want.typeName, c.have.typeName); err != "" {
			t.Error("mismatched field parent type:", err)
		}
	}
}

func stringsEqual(want, have string) string {
	if want != have {
		return fmt.Sprintf("mismatched values:\nwant: %q\nhave: %q", want, have)
	}

	return ""
}

type (
	queryVarResolver struct{}
	filterArgs       struct {
		Required string
		Optional *string
	}
)

type filterSearchResults struct {
	Match *string
}

func (r *queryVarResolver) Search(ctx context.Context, args *struct{ Filter filterArgs }) []filterSearchResults {
	return []filterSearchResults{}
}

func TestQueryVariablesValidation(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{{
		Schema: graphql.MustParseSchema(`
			input SearchFilter {
			  	required: String!
			  	optional: String
			}

			type SearchResults {
				match: String
			}

			type Query {
				search(filter: SearchFilter!): [SearchResults!]!
			}`, &queryVarResolver{}, graphql.UseFieldResolvers()),
		Query: `
        		query {
        			search(filter: {}) {
        				match
        			}
        		}`,
		ExpectedErrors: []*gqlerrors.QueryError{{
			Message:   "Argument \"filter\" has invalid value {}.\nIn field \"required\": Expected \"String!\", found null.",
			Locations: []gqlerrors.Location{{Line: 3, Column: 27}},
			Rule:      "ArgumentsOfCorrectType",
		}},
	}, {
		Schema: graphql.MustParseSchema(`
			input SearchFilter {
				required: String!
				optional: String
			}

			type SearchResults {
				match: String
			}

			type Query {
				search(filter: SearchFilter!): [SearchResults!]!
			}`, &queryVarResolver{}, graphql.UseFieldResolvers()),
		Query: `
			query q($filter: SearchFilter!) {
				search(filter: $filter) {
					match
				}
			}`,
		Variables: map[string]interface{}{"filter": map[string]interface{}{}},
		ExpectedErrors: []*gqlerrors.QueryError{{
			Message:   "Variable \"required\" has invalid value null.\nExpected type \"String!\", found null.",
			Locations: []gqlerrors.Location{{Line: 3, Column: 5}},
			Rule:      "VariablesOfCorrectType",
		}},
	}})
}

type (
	interfaceImplementingInterfaceResolver struct{}
	interfaceImplementingInterfaceExample  struct {
		A string
		B string
		C bool
	}
)

func (r *interfaceImplementingInterfaceResolver) Hey() *interfaceImplementingInterfaceExample {
	return &interfaceImplementingInterfaceExample{
		A: "testing",
		B: "test",
		C: true,
	}
}

func TestInterfaceImplementingInterface(t *testing.T) {
	gqltesting.RunTests(t, []*gqltesting.Test{{
		Schema: graphql.MustParseSchema(`
        interface A {
          a: String!
        }
        interface B implements A {
          a: String!
          b: String!
        }
        interface C implements B & A {
          a: String!
          b: String!
          c: Boolean!
        }
        type ABC implements C {
          a: String!
          b: String!
          c: Boolean!
        }
        type Query {
          hey: ABC
        }`, &interfaceImplementingInterfaceResolver{}, graphql.UseFieldResolvers(), graphql.UseFieldResolvers()),
		Query: `query {hey { a b c }}`,
		ExpectedResult: `
				{
					"hey": {
						"a": "testing",
						"b": "test",
						"c": true
					}
				}
			`,
	}})
}

func TestCircularFragmentMaxDepth(t *testing.T) {
	withMaxDepth := graphql.MustParseSchema(starwars.Schema, &starwars.Resolver{}, graphql.MaxDepth(2))
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: withMaxDepth,
			Query: `
	              query {
	                  ...X
	              }

	              fragment X on Query {
	                  ...Y
	              }
	              fragment Y on Query {
	                  ...X
	              }
	          `,
			ExpectedErrors: []*gqlerrors.QueryError{{
				Message: `Cannot spread fragment "X" within itself via "Y".`,
				Rule:    "NoFragmentCyclesRule",
				Locations: []gqlerrors.Location{
					{Line: 7, Column: 20},
					{Line: 10, Column: 20},
				},
			}},
		},
	})
}

func TestMaxQueryLength(t *testing.T) {
	withMaxQueryLen := graphql.MustParseSchema(starwars.Schema, &starwars.Resolver{}, graphql.MaxQueryLength(75))
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: withMaxQueryLen,
			// Query length is 69 bytes
			Query: `
				query {
					hero(episode: EMPIRE) {
						name
					}
				}
			`,
			ExpectedResult: `{"hero":{"name":"Luke Skywalker"}}`,
		},
		{
			Schema: withMaxQueryLen,
			Query: `
				query HeroForEpisode {
					hero(episode: WRATH_OF_KHAN) {
						name
					}
				}
			`,
			ExpectedErrors: []*gqlerrors.QueryError{{
				Message: `query length 91 exceeds the maximum allowed query length of 75 bytes`,
			}},
		},
	})
}

type (
	RootResolver         struct{}
	QueryResolver        struct{}
	MutationResolver     struct{}
	SubscriptionResolver struct {
		err      error
		upstream <-chan *helloEventResolver
	}
)

func (r *RootResolver) Query() *QueryResolver {
	return &QueryResolver{}
}

func (r *RootResolver) Mutation() *MutationResolver {
	return &MutationResolver{}
}

type helloEventResolver struct {
	msg string
	err error
}

func (r *helloEventResolver) Msg() (string, error) {
	return r.msg, r.err
}

func closedHelloEventUpstream(rr ...*helloEventResolver) <-chan *helloEventResolver {
	c := make(chan *helloEventResolver, len(rr))
	for _, r := range rr {
		c <- r
	}
	close(c)
	return c
}

func (r *RootResolver) Subscription() *SubscriptionResolver {
	return &SubscriptionResolver{
		upstream: closedHelloEventUpstream(
			&helloEventResolver{msg: "Hello subscription!"},
			&helloEventResolver{err: errors.New("resolver error")},
			&helloEventResolver{msg: "Hello again!"},
		),
	}
}

func (qr *QueryResolver) Hello() string {
	return "Hello query!"
}

func (mr *MutationResolver) Hello() string {
	return "Hello mutation!"
}

func (sr *SubscriptionResolver) Hello(ctx context.Context) (chan *helloEventResolver, error) {
	if sr.err != nil {
		return nil, sr.err
	}

	c := make(chan *helloEventResolver)
	go func() {
		for r := range sr.upstream {
			select {
			case <-ctx.Done():
				close(c)
				return
			case c <- r:
			}
		}
		close(c)
	}()

	return c, nil
}

type errRootResolver1 struct {
	RootResolver
}

// Query is invalid because it doesn't have a return value.
func (*errRootResolver1) Query() {}

type errRootResolver2 struct {
	RootResolver
}

// Query is invalid because it has more than 1 return value
func (*errRootResolver2) Query() (*QueryResolver, error) {
	return nil, nil
}

type errRootResolver3 struct {
	RootResolver
}

// Mutation is invalid because it returns nil
func (*errRootResolver3) Mutation() *MutationResolver {
	return nil
}

type errRootResolver4 struct {
	RootResolver
}

// Query is invalid because it doesn't return a pointer.
func (*errRootResolver4) Query() MutationResolver {
	return MutationResolver{}
}

type errRootResolver5 struct {
	RootResolver
}

// Query is invalid because it returns *[]int instead of a resolver.
func (*errRootResolver5) Query() *[]int {
	return &[]int{1, 2}
}

type errRootResolver6 struct {
	RootResolver
}

// Mutation is invalid because it returns a map[string]int instead of a resolver.
func (*errRootResolver6) Mutation() map[string]int {
	return map[string]int{"key": 3}
}

type errRootResolver7 struct {
	RootResolver
}

// Subscription is invalid because it returns an invalid resolver.
func (*errRootResolver7) Subscription() interface{} {
	a := struct {
		Name string
	}{Name: "invalid"}
	return &a
}

type errRootResolver8 struct {
	RootResolver
}

// Query is invalid because it accepts arguments.
func (*errRootResolver8) Query(ctx context.Context) *QueryResolver {
	return &QueryResolver{}
}

// TestSeparateResolvers ensures that a field with the same name is allowed in different operations
func TestSeparateResolvers(t *testing.T) {
	helloEverywhere := `
		schema {
			query: Query
			mutation: Mutation
			subscription: Subscription
		}

		type Query {
			hello: String!
		}

		type Mutation {
			hello: String!
		}

		type Subscription {
			hello: HelloEvent!
		}

		type HelloEvent {
			msg: String!
		}
	`

	separateSchema := graphql.MustParseSchema(helloEverywhere, &RootResolver{})

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: separateSchema,
			Query: `
				query {
					hello
				}
			`,
			ExpectedResult: `
				{
					"hello": "Hello query!"
				}
			`,
		},
		{
			Schema: separateSchema,
			Query: `
				mutation {
					hello
				}
			`,
			ExpectedResult: `
				{
					"hello": "Hello mutation!"
				}
			`,
		},
	})

	gqltesting.RunSubscribes(t, []*gqltesting.TestSubscription{
		{
			Name:   "ok",
			Schema: separateSchema,
			Query: `
				subscription {
					hello {
						msg
					}
				}
			`,
			ExpectedResults: []gqltesting.TestResponse{
				{
					Data: json.RawMessage(`
						{
							"hello": {
								"msg": "Hello subscription!"
							}
						}
					`),
				},
				{
					// null propagates all the way up because msg is non-null
					Data:   json.RawMessage(`null`),
					Errors: []*gqlerrors.QueryError{gqlerrors.Errorf("%s", errResolver)},
				},
				{
					Data: json.RawMessage(`
						{
							"hello": {
								"msg": "Hello again!"
							}
						}
					`),
				},
			},
		},
	})

	// test errors with invalid resolvers
	tests := []struct {
		name     string
		resolver interface{}
		opts     []graphql.SchemaOpt
		wantErr  string
	}{
		{
			name:     "query_method_has_no_return_val",
			resolver: &errRootResolver1{},
			wantErr:  "method \"Query\" of *graphql_test.errRootResolver1 must have 1 return value, got 0",
		},
		{
			name:     "query_method_returns_too_many_vals",
			resolver: &errRootResolver2{},
			wantErr:  "method \"Query\" of *graphql_test.errRootResolver2 must have 1 return value, got 2",
		},
		{
			name:     "mutation_method_returns_nil",
			resolver: &errRootResolver3{},
			wantErr:  "method \"Mutation\" of *graphql_test.errRootResolver3 must return a non-nil result, got <nil>",
		},
		{
			name:     "query_method_does_not_return_a_pointer",
			resolver: &errRootResolver4{},
			wantErr:  "method \"Query\" of *graphql_test.errRootResolver4 must return an interface or a pointer, got graphql_test.MutationResolver",
		},
		{
			name:     "query_method_returns_invalid_resolver_type",
			resolver: &errRootResolver5{},
			wantErr:  "*[]int does not resolve \"Query\": missing method for field \"hello\"",
		},
		{
			name:     "mutation_method_returns_invalid_resolver_type",
			resolver: &errRootResolver6{},
			wantErr:  "method \"Mutation\" of *graphql_test.errRootResolver6 must return an interface or a pointer, got map[string]int",
		},
		{
			name:     "query_subscription_returns_invalid_resolver_type",
			resolver: &errRootResolver7{},
			wantErr:  "*struct { Name string } does not resolve \"Subscription\": missing method for field \"hello\"",
		},
		{
			name:     "mutation_method_returns_invalid_resolver_type",
			resolver: &errRootResolver8{},
			wantErr:  "method \"Query\" of *graphql_test.errRootResolver8 must not accept any arguments, got 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := graphql.ParseSchema(helloEverywhere, tt.resolver, tt.opts...)
			if err == nil {
				t.Fatalf("want err: %q, got: <nil>", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("want err: %q, got: %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestSchemaExtension(t *testing.T) {
	t.Parallel()

	sdl := `
	directive @awesome on SCHEMA

	schema {
		query: Query
	}

	type Query {
		hello: String!
	}
	
	extend schema @awesome
	`
	schema := graphql.MustParseSchema(sdl, &helloResolver{})

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: schema,
			Query: `
				{
					hello
				}
			`,
			ExpectedResult: `
				{
					"hello": "Hello world!"
				}
			`,
		},
	})

	ast := schema.AST()
	dirs := ast.SchemaDefinition.Directives
	if len(dirs) != 1 {
		t.Fatalf("expected 1 schema directive, got %d", len(dirs))
	}
	name := dirs[0].Name.Name
	if name != "awesome" {
		t.Fatalf(`expected an "awesome" schema directive, got %q`, dirs[0].Name.Name)
	}
}

func TestGraphqlNames(t *testing.T) {
	t.Parallel()

	sdl1 := `
	type Query {
		hello: String!
	}
	`
	type invalidResolver1 struct {
		Field1 string `graphql:"hello"`
		Field2 string `graphql:"hello"`
	}

	wantErr := fmt.Errorf(`*graphql_test.invalidResolver1 does not resolve "Query": multiple fields have a graphql reflect tag "hello"`)
	_, err := graphql.ParseSchema(sdl1, &invalidResolver1{}, graphql.UseFieldResolvers())
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("want err %q, got %q", wantErr, err)
	}

	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(`
				type Query {
					_hello: String!
					hello: String!
					Hello: String!
					HELLO: String!
				}`,
				func() interface{} {
					type helloTagResolver struct {
						Hello           string
						HelloUnderscore string `graphql:"_hello"`
						HelloLower      string `graphql:"hello"`
						HelloTitle      string `graphql:"Hello"`
						HelloUpper      string `graphql:"HELLO"`
					}
					return &helloTagResolver{
						Hello:           "This field will not be used during query execution!",
						HelloLower:      "Hello, graphql!",
						HelloTitle:      "Hello, GraphQL!",
						HelloUnderscore: "Hello, _!",
						HelloUpper:      "Hello, GRAPHQL!",
					}
				}(),
				graphql.UseFieldResolvers()),
			Query: `
				{
					_hello
					hello
					Hello
					HELLO
				}
			`,
			ExpectedResult: `
				{
					"_hello": "Hello, _!",
				    "hello": "Hello, graphql!",
				    "Hello": "Hello, GraphQL!",
				    "HELLO": "Hello, GRAPHQL!"
				}
			`,
		},
	})
}

func Test_fieldFunc(t *testing.T) {
	sdl := `
		type Query {
			hello(name: String!): String!
		}
	`
	gqltesting.RunTests(t, []*gqltesting.Test{
		{
			Schema: graphql.MustParseSchema(sdl,
				func() interface{} {
					type helloTagResolver struct {
						Hello func(args struct{ Name string }) string
					}
					fn := func(args struct{ Name string }) string {
						return "Hello, " + args.Name + "!"
					}
					return &helloTagResolver{
						Hello: fn,
					}
				}(),
				graphql.UseFieldResolvers()),
			Query: `
				{
					hello(name: "GraphQL")
				}
			`,
			ExpectedResult: `
				{
				    "hello": "Hello, GraphQL!"
				}
			`,
		},
		{
			Schema: graphql.MustParseSchema(sdl,
				func() interface{} {
					type helloTagResolver struct {
						Greet func(ctx context.Context, args struct{ Name string }) (string, error) `graphql:"hello"`
					}
					fn := func(_ context.Context, args struct{ Name string }) (string, error) {
						return "Hello, " + args.Name + "!", nil
					}
					return &helloTagResolver{
						Greet: fn,
					}
				}(),
				graphql.UseFieldResolvers()),
			Query: `
				{
					hello(name: "GraphQL")
				}
			`,
			ExpectedResult: `
				{
				    "hello": "Hello, GraphQL!"
				}
			`,
		},
	})
}
