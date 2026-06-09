package auth

import (
	"context"
	"log/slog"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
)

// GetUsuario implements portin.GetUsuarioUseCase.
type GetUsuario struct {
	usuarios portout.UsuarioRepository
}

// NewGetUsuario creates a new GetUsuario use case.
func NewGetUsuario(u portout.UsuarioRepository) *GetUsuario {
	return &GetUsuario{usuarios: u}
}

// Execute returns a user by ID.
func (uc *GetUsuario) Execute(ctx context.Context, id string) (*domain.Usuario, error) {
	slog.InfoContext(ctx, "get usuario", "id", id)
	return uc.usuarios.FindByID(ctx, id)
}

// UpdateUsuario implements portin.UpdateUsuarioUseCase.
type UpdateUsuario struct {
	usuarios portout.UsuarioRepository
}

// NewUpdateUsuario creates a new UpdateUsuario use case.
func NewUpdateUsuario(u portout.UsuarioRepository) *UpdateUsuario {
	return &UpdateUsuario{usuarios: u}
}

// Execute updates an existing user profile.
func (uc *UpdateUsuario) Execute(ctx context.Context, usuarioID string, in portin.UpdateUsuarioInput) (*domain.Usuario, error) {
	slog.InfoContext(ctx, "update usuario", "id", usuarioID)

	usuario, err := uc.usuarios.FindByID(ctx, usuarioID)
	if err != nil {
		return nil, err
	}

	if in.Nome != "" {
		usuario.Nome = in.Nome
	}
	if in.Email != "" {
		usuario.Email = in.Email
	}
	if in.CPF != "" {
		usuario.CPF = in.CPF
	}
	if in.FotoPerfil != "" {
		usuario.FotoPerfil = in.FotoPerfil
	}
	if in.Telefone != nil {
		usuario.Telefone = in.Telefone
	}
	if in.Endereco != nil {
		usuario.Endereco = in.Endereco
	}

	return uc.usuarios.Update(ctx, usuario)
}

// ListUsuarios implements portin.ListUsuariosUseCase.
type ListUsuarios struct {
	usuarios portout.UsuarioRepository
}

// NewListUsuarios creates a new ListUsuarios use case.
func NewListUsuarios(u portout.UsuarioRepository) *ListUsuarios {
	return &ListUsuarios{usuarios: u}
}

// Execute returns a paginated list of users.
func (uc *ListUsuarios) Execute(ctx context.Context, page, pageSize int) ([]domain.Usuario, int64, error) {
	slog.InfoContext(ctx, "list usuarios", "page", page, "pageSize", pageSize)
	return uc.usuarios.FindAll(ctx, page, pageSize)
}

// UpdateUserRole implements portin.UpdateUserRoleUseCase.
type UpdateUserRole struct {
	usuarios portout.UsuarioRepository
}

// NewUpdateUserRole creates a new UpdateUserRole use case.
func NewUpdateUserRole(u portout.UsuarioRepository) *UpdateUserRole {
	return &UpdateUserRole{usuarios: u}
}

// Execute updates the roles of a user.
func (uc *UpdateUserRole) Execute(ctx context.Context, in portin.UpdateUserRoleInput) (*domain.Usuario, error) {
	slog.InfoContext(ctx, "update user role", "usuarioId", in.UsuarioID)
	usuario, err := uc.usuarios.FindByID(ctx, in.UsuarioID)
	if err != nil {
		return nil, err
	}
	usuario.Roles = in.Roles
	return uc.usuarios.Update(ctx, usuario)
}
