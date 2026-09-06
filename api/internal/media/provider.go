package media

import (
	"context"
	"log"
	"net/url"
	"time"
)

// Provider é um mecanismo capaz de resolver e baixar mídia. Hoje existe uma
// implementação (yt-dlp); amanhã pode existir uma por plataforma.
//
// A interface é curta de propósito. Formatos vêm dentro de Metadata porque
// pedi-los à parte dobraria a ida à plataforma, e não há Cancel porque o
// cancelamento é o context — que é como Go já expressa isso.
type Provider interface {
	// Name identifica o provider em logs, no histórico e no painel.
	Name() string

	// CanHandle diz se este provider sabe lidar com a URL. Recebe a forma já
	// normalizada, nunca a string crua do usuário.
	CanHandle(parsed *url.URL) bool

	// Metadata resolve o conteúdo sem baixá-lo.
	Metadata(ctx context.Context, parsed *url.URL, opts MetadataOptions) (*Metadata, error)

	// Download escreve o arquivo em req.OutputDir. Cancelar o ctx encerra o
	// trabalho e o processo filho.
	Download(ctx context.Context, req Request, onProgress ProgressFunc) (*Result, error)

	// Health verifica se o provider e suas dependências estão utilizáveis.
	Health(ctx context.Context) Health
}

// MetadataOptions carrega o que é opcional na resolução. Existe como struct
// para acrescentar um campo não virar mudança de assinatura em toda
// implementação de Provider.
type MetadataOptions struct {
	// CookieFile é um arquivo temporário de sessão. Algumas plataformas — o
	// Vimeo é o caso claro — recusam até a LEITURA de metadados sem sessão, e
	// não só o download. O provider nunca deve copiá-lo nem registrá-lo em log.
	CookieFile string
}

// Registry escolhe o provider de cada URL e é o ponto único de extensão: somar
// um provider novo é registrá-lo aqui, sem tocar em handler, job ou banco.
type Registry struct {
	// providers está em ordem de preferência. O primeiro que aceitar a URL
	// atende; os seguintes servem de fallback quando ele falha.
	providers []Provider
}

func NewRegistry(providers ...Provider) *Registry {
	return &Registry{providers: providers}
}

// Providers devolve a lista registrada, para diagnóstico.
func (r *Registry) Providers() []Provider {
	return r.providers
}

// candidatos devolve, em ordem, os providers que aceitam a URL.
func (r *Registry) candidatos(parsed *url.URL) []Provider {
	var aceitos []Provider
	for _, provider := range r.providers {
		if provider.CanHandle(parsed) {
			aceitos = append(aceitos, provider)
		}
	}
	return aceitos
}

// Resolve identifica plataforma e provider de uma URL crua, sem contatar a
// plataforma. É o que a API usa para recusar cedo o que ninguém sabe atender.
func (r *Registry) Resolve(bruta string) (Platform, Provider, string, error) {
	normalizada, parsed, err := NormalizeURL(bruta)
	if err != nil {
		return PlatformUnknown, nil, "", err
	}

	plataforma := PlatformFor(parsed)
	if !plataforma.TemSuporte() {
		return plataforma, nil, normalizada, &Error{
			Kind:   ErrUnsupportedPlatform,
			Detail: plataforma.Label() + " não é suportado pelo mecanismo de download",
		}
	}

	aceitos := r.candidatos(parsed)
	if len(aceitos) == 0 {
		return plataforma, nil, normalizada, wrap(ErrUnsupportedPlatform, parsed.Hostname())
	}

	return plataforma, aceitos[0], normalizada, nil
}

// Metadata resolve o conteúdo, tentando os providers em ordem. O fallback é
// mecanismo de disponibilidade: um provider fora do ar não pode derrubar uma
// plataforma que outro sabe atender.
//
// Erros que descrevem o CONTEÚDO (privado, indisponível, ao vivo) não disparam
// fallback: o próximo provider chegaria à mesma conclusão, e insistir só
// gastaria tempo e requisições contra a plataforma.
func (r *Registry) Metadata(ctx context.Context, bruta string, opts MetadataOptions) (*Metadata, string, error) {
	normalizada, parsed, err := NormalizeURL(bruta)
	if err != nil {
		return nil, "", err
	}

	if plataforma := PlatformFor(parsed); !plataforma.TemSuporte() {
		// Reconhecida, mas sem extractor. Sem esta checagem o caminho genérico
		// baixaria a página e só então falharia — dezenas de segundos para
		// chegar à mesma conclusão que já se sabe aqui.
		return nil, normalizada, &Error{
			Kind:   ErrUnsupportedPlatform,
			Detail: plataforma.Label() + " não é suportado pelo mecanismo de download",
		}
	}

	aceitos := r.candidatos(parsed)
	if len(aceitos) == 0 {
		return nil, normalizada, wrap(ErrUnsupportedPlatform, parsed.Hostname())
	}

	var ultimoErro error
	for indice, provider := range aceitos {
		metadata, err := provider.Metadata(ctx, parsed, opts)
		if err == nil {
			metadata.Platform = PlatformFor(parsed)
			metadata.Provider = provider.Name()
			if metadata.FetchedAt.IsZero() {
				metadata.FetchedAt = time.Now().UTC()
			}
			return metadata, normalizada, nil
		}

		ultimoErro = err
		if !vaiTentarOutro(ctx, err) {
			break
		}
		// Só anuncia a troca quando existe mesmo um próximo provider; com um
		// só registrado, a linha diria que vai tentar algo que não existe.
		if indice+1 >= len(aceitos) {
			break
		}
		log.Printf("media: provider=%s falhou ao resolver plataforma=%s, tentando o próximo: %v",
			provider.Name(), PlatformFor(parsed), err)
	}

	return nil, normalizada, ultimoErro
}

// ProviderByName encontra um provider registrado. Usado pelo worker, que grava
// no banco qual provider atendeu e precisa do mesmo na hora de baixar.
func (r *Registry) ProviderByName(nome string) Provider {
	for _, provider := range r.providers {
		if provider.Name() == nome {
			return provider
		}
	}
	return nil
}

// ProviderFor devolve o provider preferido para uma URL já normalizada.
func (r *Registry) ProviderFor(parsed *url.URL) Provider {
	if aceitos := r.candidatos(parsed); len(aceitos) > 0 {
		return aceitos[0]
	}
	return nil
}

// Health coleta o diagnóstico de todos os providers.
func (r *Registry) Health(ctx context.Context) []Health {
	saude := make([]Health, 0, len(r.providers))
	for _, provider := range r.providers {
		saude = append(saude, provider.Health(ctx))
	}
	return saude
}

// vaiTentarOutro decide se vale acionar o próximo provider. Cancelamento e
// prazo esgotado nunca reentram: o pedido já acabou.
func vaiTentarOutro(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}

	switch UserMessage(err) {
	case ErrContentUnavailable.Error(),
		ErrContentPrivate.Error(),
		ErrAuthRequired.Error(),
		ErrGeoBlocked.Error(),
		ErrLiveContent.Error(),
		ErrInvalidURL.Error(),
		ErrCanceled.Error(),
		ErrTimeout.Error():
		return false
	}
	return true
}
