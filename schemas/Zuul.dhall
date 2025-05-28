{ Type =
    { defaultAuthenticator : Optional Text
    , elasticsearchconns : Optional (List (./ZuulElasticsearchConn.dhall).Type)
    , executor : Optional (./ZuulExecutor.dhall).Type
    , gerritconns : Optional (List (./ZuulGerritConn.dhall).Type)
    , gitconns : Optional (List (./ZuulGitConn.dhall).Type)
    , githubconns : Optional (List (./ZuulGithubConn.dhall).Type)
    , gitlabconns : Optional (List (./ZuulGitlabConn.dhall).Type)
    , merger : Optional (./ZuulMerger.dhall).Type
    , oidcAuthenticators : Optional (List (./ZuulOidcAuthenticators.dhall).Type)
    , pagureconns : Optional (List (./ZuulPagureConn.dhall).Type)
    , scheduler : Optional (./ZuulScheduler.dhall).Type
    , smtpconns : Optional (List (./ZuulSmtpConn.dhall).Type)
    , web : Optional (./ZuulWeb.dhall).Type
    }
, default =
  { defaultAuthenticator = None Text
  , elasticsearchconns = None (List (./ZuulElasticsearchConn.dhall).Type)
  , executor = None (./ZuulExecutor.dhall).Type
  , gerritconns = None (List (./ZuulGerritConn.dhall).Type)
  , gitconns = None (List (./ZuulGitConn.dhall).Type)
  , githubconns = None (List (./ZuulGithubConn.dhall).Type)
  , gitlabconns = None (List (./ZuulGitlabConn.dhall).Type)
  , merger = None (./ZuulMerger.dhall).Type
  , oidcAuthenticators = None (List (./ZuulOidcAuthenticators.dhall).Type)
  , pagureconns = None (List (./ZuulPagureConn.dhall).Type)
  , scheduler = None (./ZuulScheduler.dhall).Type
  , smtpconns = None (List (./ZuulSmtpConn.dhall).Type)
  , web = None (./ZuulWeb.dhall).Type
  }
}