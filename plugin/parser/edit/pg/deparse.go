package pg

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/api"
	"github.com/bytebase/bytebase/plugin/parser"
	"github.com/bytebase/bytebase/plugin/parser/ast"
)

// DeparseContext is the context including walkthrough nodes and comment statements.
type DeparseContext struct {
	NodeList []ast.Node
	// StmtList mainly contains column comment statements.
	StmtList []string
}

// DeparseDatabaseEdit deparses DatabaseEdit to DDL statement.
func (*SchemaEditor) DeparseDatabaseEdit(databaseEdit *api.DatabaseEdit) (string, error) {
	ctx := &DeparseContext{
		NodeList: []ast.Node{},
		StmtList: []string{},
	}
	for _, createTableContext := range databaseEdit.CreateTableList {
		err := transformCreateTableContext(ctx, createTableContext)
		if err != nil {
			return "", err
		}
	}
	for _, renameTableContext := range databaseEdit.RenameTableList {
		transformRenameTableContext(ctx, renameTableContext)
	}
	for _, alterTableContext := range databaseEdit.AlterTableList {
		err := transformAlterTableContext(ctx, alterTableContext)
		if err != nil {
			return "", err
		}
	}
	for _, dropTableContext := range databaseEdit.DropTableList {
		transformDropTableContext(ctx, dropTableContext)
	}

	var stmtList []string
	for _, node := range ctx.NodeList {
		stmt, err := restoreASTNode(node)
		if err != nil {
			return "", err
		}
		stmtList = append(stmtList, stmt)
	}
	stmtList = append(stmtList, ctx.StmtList...)
	return strings.Join(stmtList, "\n"), nil
}

func transformCreateTableContext(ctx *DeparseContext, createTableContext *api.CreateTableContext) error {
	table := ast.TableDef{
		Type:   ast.TableTypeBaseTable,
		Schema: createTableContext.Schema,
		Name:   createTableContext.Name,
	}
	createTableStmt := &ast.CreateTableStmt{
		Name:           &table,
		ColumnList:     []*ast.ColumnDef{},
		ConstraintList: []*ast.ConstraintDef{},
	}

	for _, addColumnContext := range createTableContext.AddColumnList {
		columnDef, err := transformAddColumnContext(ctx, addColumnContext)
		if err != nil {
			return err
		}
		createTableStmt.ColumnList = append(createTableStmt.ColumnList, columnDef)

		// TODO(steven): remove this after our pg parser supports comment stmt.
		if addColumnContext.Comment != "" {
			commemtStmt := fmt.Sprintf("COMMENT ON COLUMN %s.%s IS '%s';", table.Name, columnDef.ColumnName, addColumnContext.Comment)
			ctx.StmtList = append(ctx.StmtList, commemtStmt)
		}
	}

	if len(createTableContext.PrimaryKeyList) > 0 {
		constraint := ast.ConstraintDef{
			Type:    ast.ConstraintTypePrimary,
			KeyList: []string{},
		}
		constraint.KeyList = append(constraint.KeyList, createTableContext.PrimaryKeyList...)
		createTableStmt.ConstraintList = append(createTableStmt.ConstraintList, &constraint)
	}

	for _, addForeignKeyContext := range createTableContext.AddForeignKeyList {
		foreignKeyDef := ast.ForeignDef{
			Table: &ast.TableDef{
				Type:   ast.TableTypeBaseTable,
				Schema: addForeignKeyContext.ReferencedSchema,
				Name:   addForeignKeyContext.ReferencedTable,
			},
			ColumnList: addForeignKeyContext.ReferencedColumnList,
			MatchType:  ast.ForeignMatchTypeSimple,
			OnUpdate:   &ast.ReferentialActionDef{Type: ast.ReferentialActionTypeNoAction},
			OnDelete:   &ast.ReferentialActionDef{Type: ast.ReferentialActionTypeNoAction},
		}
		constraint := ast.ConstraintDef{
			Type:    ast.ConstraintTypeForeign,
			KeyList: addForeignKeyContext.ColumnList,
			Foreign: &foreignKeyDef,
		}
		createTableStmt.ConstraintList = append(createTableStmt.ConstraintList, &constraint)
	}

	ctx.NodeList = append(ctx.NodeList, createTableStmt)
	return nil
}

func transformAlterTableContext(ctx *DeparseContext, alterTableContext *api.AlterTableContext) error {
	table := &ast.TableDef{
		Type:   ast.TableTypeBaseTable,
		Schema: alterTableContext.Schema,
		Name:   alterTableContext.Name,
	}
	alterTableStmt := &ast.AlterTableStmt{
		Table:         table,
		AlterItemList: []ast.Node{},
	}

	for _, dropColumnContext := range alterTableContext.DropColumnList {
		dropColumnStmt := ast.DropColumnStmt{
			Table:      table,
			ColumnName: dropColumnContext.Name,
		}
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &dropColumnStmt)
	}

	for _, addColumnContext := range alterTableContext.AddColumnList {
		columnDef, err := transformAddColumnContext(ctx, addColumnContext)
		if err != nil {
			return err
		}
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.AddColumnListStmt{
			Table:      table,
			ColumnList: []*ast.ColumnDef{columnDef},
		})
		if addColumnContext.Comment != "" {
			commemtStmt := fmt.Sprintf("COMMENT ON COLUMN %s.%s IS '%s';", table.Name, columnDef.ColumnName, addColumnContext.Comment)
			ctx.StmtList = append(ctx.StmtList, commemtStmt)
		}
	}

	for _, changeColumnContext := range alterTableContext.ChangeColumnList {
		if changeColumnContext.OldName != changeColumnContext.NewName {
			alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.RenameColumnStmt{
				Table:      table,
				ColumnName: changeColumnContext.OldName,
				NewName:    changeColumnContext.NewName,
			})
		}
		columnType, err := transformColumnType(changeColumnContext.Type)
		if err != nil {
			return errors.Wrap(err, "failed to transform column type")
		}
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.AlterColumnTypeStmt{
			Table:      table,
			ColumnName: changeColumnContext.NewName,
			Type:       columnType,
		})
		if changeColumnContext.Nullable {
			alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.DropNotNullStmt{
				Table:      table,
				ColumnName: changeColumnContext.NewName,
			})
		} else {
			alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.SetNotNullStmt{
				Table:      table,
				ColumnName: changeColumnContext.NewName,
			})
		}
		if changeColumnContext.Default == nil {
			alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.DropDefaultStmt{
				Table:      table,
				ColumnName: changeColumnContext.NewName,
			})
		} else {
			expression := ast.UnconvertedExpressionDef{}
			expression.SetText(*changeColumnContext.Default)
			alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.SetDefaultStmt{
				Table:      table,
				ColumnName: changeColumnContext.NewName,
				Expression: &expression,
			})
		}
		if changeColumnContext.Comment != "" {
			commemtStmt := fmt.Sprintf("COMMENT ON COLUMN %s.%s IS '%s';", table.Name, changeColumnContext.NewName, changeColumnContext.Comment)
			ctx.StmtList = append(ctx.StmtList, commemtStmt)
		}
	}

	for _, dropPrimaryKey := range alterTableContext.DropPrimaryKeyList {
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &ast.DropConstraintStmt{
			Table:          table,
			ConstraintName: dropPrimaryKey,
			IfExists:       true,
		})
	}

	if alterTableContext.PrimaryKeyList != nil && len(*alterTableContext.PrimaryKeyList) != 0 {
		constraint := ast.ConstraintDef{
			Type:    ast.ConstraintTypePrimary,
			KeyList: []string{},
		}
		constraint.KeyList = append(constraint.KeyList, *alterTableContext.PrimaryKeyList...)
		addConstraintSmt := ast.AddConstraintStmt{
			Table:      table,
			Constraint: &constraint,
		}
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &addConstraintSmt)
	}

	for _, dropForeignKey := range alterTableContext.DropForeignKeyList {
		dropConstraintStmt := ast.DropConstraintStmt{
			Table:          table,
			ConstraintName: dropForeignKey,
			IfExists:       true,
		}
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &dropConstraintStmt)
	}

	for _, addForeignKeyContext := range alterTableContext.AddForeignKeyList {
		foreignKeyDef := ast.ForeignDef{
			Table: &ast.TableDef{
				Schema: addForeignKeyContext.ReferencedSchema,
				Name:   addForeignKeyContext.ReferencedTable,
			},
			ColumnList: addForeignKeyContext.ReferencedColumnList,
			MatchType:  ast.ForeignMatchTypeSimple,
			OnUpdate:   &ast.ReferentialActionDef{Type: ast.ReferentialActionTypeNoAction},
			OnDelete:   &ast.ReferentialActionDef{Type: ast.ReferentialActionTypeNoAction},
		}
		constraint := ast.ConstraintDef{
			Type:    ast.ConstraintTypeForeign,
			KeyList: addForeignKeyContext.ColumnList,
			Foreign: &foreignKeyDef,
		}
		addConstraintSmt := ast.AddConstraintStmt{
			Table:      table,
			Constraint: &constraint,
		}
		alterTableStmt.AlterItemList = append(alterTableStmt.AlterItemList, &addConstraintSmt)
	}

	ctx.NodeList = append(ctx.NodeList, alterTableStmt)
	return nil
}

func transformRenameTableContext(ctx *DeparseContext, renameTableContext *api.RenameTableContext) {
	table := ast.TableDef{
		Type:   ast.TableTypeBaseTable,
		Schema: renameTableContext.Schema,
		Name:   renameTableContext.OldName,
	}
	alterTableStmt := &ast.AlterTableStmt{
		Table: &table,
		AlterItemList: []ast.Node{
			&ast.RenameTableStmt{
				Table:   &table,
				NewName: renameTableContext.NewName,
			},
		},
	}
	ctx.NodeList = append(ctx.NodeList, alterTableStmt)
}

func transformDropTableContext(ctx *DeparseContext, dropTableContext *api.DropTableContext) {
	dropTableStmt := &ast.DropTableStmt{
		IfExists: true,
		TableList: []*ast.TableDef{
			{
				Type:   ast.TableTypeBaseTable,
				Schema: dropTableContext.Schema,
				Name:   dropTableContext.Name,
			},
		},
	}
	ctx.NodeList = append(ctx.NodeList, dropTableStmt)
}

func transformAddColumnContext(_ *DeparseContext, addColumnContext *api.AddColumnContext) (*ast.ColumnDef, error) {
	columnType, err := transformColumnType(addColumnContext.Type)
	if err != nil {
		return nil, errors.Wrap(err, "failed to transform column type")
	}

	columnDef := &ast.ColumnDef{
		ColumnName: addColumnContext.Name,
		Type:       columnType,
	}

	var constraintList []*ast.ConstraintDef
	if addColumnContext.Default != nil {
		expression := ast.UnconvertedExpressionDef{}
		expression.SetText(*addColumnContext.Default)
		constraintList = append(constraintList, &ast.ConstraintDef{
			Type:       ast.ConstraintTypeDefault,
			KeyList:    []string{addColumnContext.Name},
			Expression: &expression,
		})
	}
	if !addColumnContext.Nullable {
		constraintList = append(constraintList, &ast.ConstraintDef{
			Type:    ast.ConstraintTypeNotNull,
			KeyList: []string{addColumnContext.Name},
		})
	}
	columnDef.ConstraintList = constraintList

	return columnDef, nil
}

func transformColumnType(typeStr string) (ast.DataType, error) {
	// Mock a CreateTableStmt with type string to get the actually types.FieldType.
	stmt := fmt.Sprintf(`CREATE TABLE "public"."column_type" ("column_type" %s);`, typeStr)
	nodeList, err := parser.Parse(parser.Postgres, parser.ParseContext{}, stmt)
	if err != nil {
		return nil, err
	}
	if len(nodeList) != 1 {
		return nil, errors.Errorf("expect node list length to be 1, get %d", len(nodeList))
	}

	node, ok := nodeList[0].(*ast.CreateTableStmt)
	if !ok {
		return nil, errors.New("expect the type of the node to be CreateTableStmt")
	}
	if len(node.ColumnList) != 1 {
		return nil, errors.Errorf("expect node list length to be 1, get %d", len(nodeList))
	}

	col := node.ColumnList[0]
	return col.Type, nil
}

func restoreASTNode(node ast.Node) (string, error) {
	stmt, err := parser.Deparse(parser.Postgres, parser.DeparseContext{}, node)
	if err != nil {
		return "", err
	}
	return stmt, nil
}
