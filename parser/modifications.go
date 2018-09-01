package parser

import (
	"go/ast"
	"go/token"
)

// Parse every function as an empty list of statements.
func (p *parser) parseBody(scope *ast.Scope) *ast.BlockStmt {
	if p.trace {
		defer un(trace(p, "Body"))
	}

	lbrace := p.expect(token.LBRACE)
	braceCount := 0
	for {
		if p.tok == token.LBRACE {
			braceCount++
		}

		if p.tok == token.RBRACE {
			braceCount--
			if braceCount == -1 {
				break
			}
		}

		if p.tok == token.EOF {
			break
		}
		p.next()
	}

	rbrace := p.expect(token.RBRACE)
	_ = lbrace
	_ = rbrace

	return &ast.BlockStmt{Lbrace: lbrace, List: nil, Rbrace: rbrace}
}
