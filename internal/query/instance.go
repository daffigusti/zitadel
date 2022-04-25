package query

import (
	"context"
	"database/sql"
	errs "errors"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"golang.org/x/text/language"

	"github.com/caos/zitadel/internal/api/authz"
	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/query/projection"
)

var (
	instanceTable = table{
		name: projection.InstanceProjectionTable,
	}
	InstanceColumnID = Column{
		name:  projection.InstanceColumnID,
		table: instanceTable,
	}
	InstanceColumnName = Column{
		name:  projection.InstanceColumnName,
		table: instanceTable,
	}
	InstanceColumnCreationDate = Column{
		name:  projection.InstanceColumnCreationDate,
		table: instanceTable,
	}
	InstanceColumnChangeDate = Column{
		name:  projection.InstanceColumnChangeDate,
		table: instanceTable,
	}
	InstanceColumnSequence = Column{
		name:  projection.InstanceColumnSequence,
		table: instanceTable,
	}
	InstanceColumnGlobalOrgID = Column{
		name:  projection.InstanceColumnGlobalOrgID,
		table: instanceTable,
	}
	InstanceColumnProjectID = Column{
		name:  projection.InstanceColumnProjectID,
		table: instanceTable,
	}
	InstanceColumnConsoleID = Column{
		name:  projection.InstanceColumnConsoleID,
		table: instanceTable,
	}
	InstanceColumnConsoleAppID = Column{
		name:  projection.InstanceColumnConsoleAppID,
		table: instanceTable,
	}
	InstanceColumnSetupStarted = Column{
		name:  projection.InstanceColumnSetUpStarted,
		table: instanceTable,
	}
	InstanceColumnSetupDone = Column{
		name:  projection.InstanceColumnSetUpDone,
		table: instanceTable,
	}
	InstanceColumnDefaultLanguage = Column{
		name:  projection.InstanceColumnDefaultLanguage,
		table: instanceTable,
	}
)

type Instance struct {
	ID           string
	ChangeDate   time.Time
	CreationDate time.Time
	Sequence     uint64

	GlobalOrgID     string
	IAMProjectID    string
	ConsoleID       string
	ConsoleAppID    string
	DefaultLanguage language.Tag
	SetupStarted    domain.Step
	SetupDone       domain.Step
	Host            string
}

type Instances struct {
	SearchResponse
	Instances []*Instance
}

func (i *Instance) InstanceID() string {
	return i.ID
}

func (i *Instance) ProjectID() string {
	return i.IAMProjectID
}

func (i *Instance) ConsoleClientID() string {
	return i.ConsoleID
}

func (i *Instance) ConsoleApplicationID() string {
	return i.ConsoleAppID
}

func (i *Instance) RequestedDomain() string {
	return i.Host
}

type InstanceSearchQueries struct {
	SearchRequest
	Queries []SearchQuery
}

func NewInstanceIDsListSearchQuery(ids ...string) (SearchQuery, error) {
	list := make([]interface{}, len(ids))
	for i, value := range ids {
		list[i] = value
	}
	return NewListQuery(InstanceColumnID, list, ListIn)
}

func (q *InstanceSearchQueries) toQuery(query sq.SelectBuilder) sq.SelectBuilder {
	query = q.SearchRequest.toQuery(query)
	for _, q := range q.Queries {
		query = q.toQuery(query)
	}
	return query
}

func (q *Queries) SearchInstances(ctx context.Context, queries *InstanceSearchQueries) (instances *Instances, err error) {
	query, scan := prepareInstancesQuery()
	stmt, args, err := queries.toQuery(query).ToSql()
	if err != nil {
		return nil, errors.ThrowInvalidArgument(err, "QUERY-M9fow", "Errors.Query.SQLStatement")
	}

	rows, err := q.client.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, errors.ThrowInternal(err, "QUERY-3j98f", "Errors.Internal")
	}
	instances, err = scan(rows)
	if err != nil {
		return nil, err
	}
	return instances, err
}

func (q *Queries) Instance(ctx context.Context) (*Instance, error) {
	stmt, scan := prepareInstanceQuery(authz.GetInstance(ctx).RequestedDomain())
	query, args, err := stmt.Where(sq.Eq{
		InstanceColumnID.identifier(): authz.GetInstance(ctx).InstanceID(),
	}).ToSql()
	if err != nil {
		return nil, errors.ThrowInternal(err, "QUERY-d9ngs", "Errors.Query.SQLStatement")
	}

	row := q.client.QueryRowContext(ctx, query, args...)
	return scan(row)
}

func (q *Queries) InstanceByHost(ctx context.Context, host string) (authz.Instance, error) {
	stmt, scan := prepareInstanceDomainQuery(host)
	query, args, err := stmt.Where(sq.Eq{
		InstanceDomainDomainCol.identifier(): strings.Split(host, ":")[0],
	}).ToSql()
	if err != nil {
		return nil, errors.ThrowInternal(err, "QUERY-SAfg2", "Errors.Query.SQLStatement")
	}

	row := q.client.QueryRowContext(ctx, query, args...)
	return scan(row)
}

func (q *Queries) GetDefaultLanguage(ctx context.Context) language.Tag {
	iam, err := q.Instance(ctx)
	if err != nil {
		return language.Und
	}
	return iam.DefaultLanguage
}

func prepareInstanceQuery(host string) (sq.SelectBuilder, func(*sql.Row) (*Instance, error)) {
	return sq.Select(
			InstanceColumnID.identifier(),
			InstanceColumnCreationDate.identifier(),
			InstanceColumnChangeDate.identifier(),
			InstanceColumnSequence.identifier(),
			InstanceColumnGlobalOrgID.identifier(),
			InstanceColumnProjectID.identifier(),
			InstanceColumnConsoleID.identifier(),
			InstanceColumnConsoleAppID.identifier(),
			InstanceColumnSetupStarted.identifier(),
			InstanceColumnSetupDone.identifier(),
			InstanceColumnDefaultLanguage.identifier(),
		).
			From(instanceTable.identifier()).PlaceholderFormat(sq.Dollar),
		func(row *sql.Row) (*Instance, error) {
			instance := &Instance{Host: host}
			lang := ""
			err := row.Scan(
				&instance.ID,
				&instance.CreationDate,
				&instance.ChangeDate,
				&instance.Sequence,
				&instance.GlobalOrgID,
				&instance.IAMProjectID,
				&instance.ConsoleID,
				&instance.ConsoleAppID,
				&instance.SetupStarted,
				&instance.SetupDone,
				&lang,
			)
			if err != nil {
				if errs.Is(err, sql.ErrNoRows) {
					return nil, errors.ThrowNotFound(err, "QUERY-n0wng", "Errors.IAM.NotFound")
				}
				return nil, errors.ThrowInternal(err, "QUERY-d9nw", "Errors.Internal")
			}
			instance.DefaultLanguage = language.Make(lang)
			return instance, nil
		}
}

func prepareInstancesQuery() (sq.SelectBuilder, func(*sql.Rows) (*Instances, error)) {
	return sq.Select(
			InstanceColumnID.identifier(),
			InstanceColumnCreationDate.identifier(),
			InstanceColumnChangeDate.identifier(),
			InstanceColumnSequence.identifier(),
			InstanceColumnGlobalOrgID.identifier(),
			InstanceColumnProjectID.identifier(),
			InstanceColumnConsoleID.identifier(),
			InstanceColumnConsoleAppID.identifier(),
			InstanceColumnSetupStarted.identifier(),
			InstanceColumnSetupDone.identifier(),
			InstanceColumnDefaultLanguage.identifier(),
			countColumn.identifier(),
		).From(instanceTable.identifier()).PlaceholderFormat(sq.Dollar),
		func(rows *sql.Rows) (*Instances, error) {
			instances := make([]*Instance, 0)
			var count uint64
			for rows.Next() {
				instance := new(Instance)
				lang := ""
				//TODO: Get Host
				err := rows.Scan(
					&instance.ID,
					&instance.CreationDate,
					&instance.ChangeDate,
					&instance.Sequence,
					&instance.GlobalOrgID,
					&instance.IAMProjectID,
					&instance.ConsoleID,
					&instance.ConsoleAppID,
					&instance.SetupStarted,
					&instance.SetupDone,
					&lang,
					&count,
				)
				if err != nil {
					return nil, err
				}
				instances = append(instances, instance)
			}

			if err := rows.Close(); err != nil {
				return nil, errors.ThrowInternal(err, "QUERY-8nlWW", "Errors.Query.CloseRows")
			}

			return &Instances{
				Instances: instances,
				SearchResponse: SearchResponse{
					Count: count,
				},
			}, nil
		}
}

func prepareInstanceDomainQuery(host string) (sq.SelectBuilder, func(*sql.Row) (*Instance, error)) {
	return sq.Select(
			InstanceColumnID.identifier(),
			InstanceColumnCreationDate.identifier(),
			InstanceColumnChangeDate.identifier(),
			InstanceColumnSequence.identifier(),
			InstanceColumnGlobalOrgID.identifier(),
			InstanceColumnProjectID.identifier(),
			InstanceColumnConsoleID.identifier(),
			InstanceColumnConsoleAppID.identifier(),
			InstanceColumnSetupStarted.identifier(),
			InstanceColumnSetupDone.identifier(),
			InstanceColumnDefaultLanguage.identifier(),
		).
			From(instanceTable.identifier()).
			LeftJoin(join(InstanceDomainInstanceIDCol, InstanceColumnID)).
			PlaceholderFormat(sq.Dollar),
		func(row *sql.Row) (*Instance, error) {
			instance := &Instance{Host: host}
			lang := ""
			err := row.Scan(
				&instance.ID,
				&instance.CreationDate,
				&instance.ChangeDate,
				&instance.Sequence,
				&instance.GlobalOrgID,
				&instance.IAMProjectID,
				&instance.ConsoleID,
				&instance.ConsoleAppID,
				&instance.SetupStarted,
				&instance.SetupDone,
				&lang,
			)
			if err != nil {
				if errs.Is(err, sql.ErrNoRows) {
					return nil, errors.ThrowNotFound(err, "QUERY-n0wng", "Errors.IAM.NotFound")
				}
				return nil, errors.ThrowInternal(err, "QUERY-d9nw", "Errors.Internal")
			}
			instance.DefaultLanguage = language.Make(lang)
			return instance, nil
		}
}