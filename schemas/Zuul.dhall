{ Type =
    { defaultAuthenticator : Optional Text
    , elasticsearchconns : Optional (List (./ElasticsearchConn.dhall).Type)
    , executor : Optional (./ZuulExecutor.dhall).Type
    , gerritconns : Optional (List (./GerritConn.dhall).Type)
    , gitconns : Optional (List (./GitConn.dhall).Type)
    , githubconns : Optional (List (./GithubConn.dhall).Type)
    , gitlabconns : Optional (List (./GitlabConn.dhall).Type)
    , merger : Optional (./ZuulMerger.dhall).Type
    , oidcAuthenticators : Optional (List (./ZuulOidcAuthenticators.dhall).Type)
    , pagureconns : Optional (List (./PagureConn.dhall).Type)
    , scheduler : Optional (./ZuulScheduler.dhall).Type
    , smtpconns : Optional (List (./SmtpConn.dhall).Type)
    , web : Optional (./ZuulWeb.dhall).Type
    }
, default =
  { defaultAuthenticator = None Text
  , elasticsearchconns = None (List (./ElasticsearchConn.dhall).Type)
  , executor = None (./ZuulExecutor.dhall).Type
  , gerritconns = None (List (./GerritConn.dhall).Type)
  , gitconns = None (List (./GitConn.dhall).Type)
  , githubconns = None (List (./GithubConn.dhall).Type)
  , gitlabconns = None (List (./GitlabConn.dhall).Type)
  , merger = None (./ZuulMerger.dhall).Type
  , oidcAuthenticators = None (List (./ZuulOidcAuthenticators.dhall).Type)
  , pagureconns = None (List (./PagureConn.dhall).Type)
  , scheduler = None (./ZuulScheduler.dhall).Type
  , smtpconns = None (List (./SmtpConn.dhall).Type)
  , web = None (./ZuulWeb.dhall).Type
  }
}