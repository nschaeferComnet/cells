package sessions

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/sessions"

	"github.com/pydio/cells/v4/common/config"
	"github.com/pydio/cells/v4/common/dao"
	"github.com/pydio/cells/v4/common/log"
	"github.com/pydio/cells/v4/common/service/frontend/sessions/securecookie"
	"github.com/pydio/cells/v4/common/service/frontend/sessions/sqlsessions"
	"github.com/pydio/cells/v4/common/service/frontend/sessions/utils"
	"github.com/pydio/cells/v4/common/sql"
	"github.com/pydio/cells/v4/common/utils/configx"
)

func NewDAO(dao dao.DAO) dao.DAO {

	timeout := config.Get("frontend", "plugin", "gui.ajax", "SESSION_TIMEOUT").Default(60).Int()
	defaultOptions := &sessions.Options{
		Path:     "/a/frontend",
		MaxAge:   60 * timeout,
		HttpOnly: true,
	}

	switch v := dao.(type) {
	case securecookie.DAO:
		ci := &cookiesImpl{}
		ci.DAO = v
		ci.storeFactory = func(u *url.URL, keyPairs ...[]byte) (sessions.Store, error) {
			cs := sessions.NewCookieStore(keyPairs...)
			cs.Options = &sessions.Options{
				Path:     defaultOptions.Path,
				MaxAge:   defaultOptions.MaxAge,
				HttpOnly: defaultOptions.HttpOnly,
			}
			if u.Scheme == "https" {
				cs.Options.Secure = true
			}
			cs.Options.Domain = u.Hostname()
			return cs, nil
		}
		return ci
	case sql.DAO:
		return &sqlsessions.Impl{
			DAO:     v,
			Options: defaultOptions,
		}
	default:
		return nil
	}
}

type DAO interface {
	dao.DAO
	GetSession(r *http.Request) (*sessions.Session, error)
	DeleteExpired(ctx context.Context, logger log.ZapLogger)
}

type cookiesImpl struct {
	dao.DAO
	sync.Mutex
	secureKeyPairs []byte
	sessionStores  map[string]sessions.Store
	storeFactory   func(u *url.URL, keyPairs ...[]byte) (sessions.Store, error)
}

func (s *cookiesImpl) Init(values configx.Values) error {
	s.sessionStores = make(map[string]sessions.Store)
	if k, e := utils.LoadKey(); e != nil {
		return e
	} else {
		s.secureKeyPairs = k
		return s.DAO.Init(values)
	}
}

func (s *cookiesImpl) GetSession(r *http.Request) (*sessions.Session, error) {
	store, er := s.storeForUrl(r.URL)
	if er != nil {
		return nil, er
	}
	return store.Get(r, utils.SessionName(r))
}

func (s *cookiesImpl) DeleteExpired(ctx context.Context, logger log.ZapLogger) {
	return
}

func (s *cookiesImpl) storeForUrl(u *url.URL) (sessions.Store, error) {
	key := u.Scheme + "://" + u.Hostname()
	s.Lock()
	defer s.Unlock()
	if ss, o := s.sessionStores[key]; o {
		return ss, nil
	}
	ss, e := s.storeFactory(u, s.secureKeyPairs)
	if e != nil {
		return nil, e
	}
	s.sessionStores[key] = ss
	return ss, nil
}
