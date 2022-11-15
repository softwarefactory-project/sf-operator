-- Regenerate using: dhall <<< https://softwarefactory-project.io/cgit/software-factory/dhall-software-factory/plain/package.dhall > sf.dhall
{ Connection =
  { Name =
      \ ( obj
        : { base-url : Optional Text
          , github-app-name : Optional Text
          , github-label : Optional Text
          , name : Text
          , type : < gerrit | git | github | pagure >
          }
        ) ->
        obj.name
  , Type =
      { base-url : Optional Text
      , github-app-name : Optional Text
      , github-label : Optional Text
      , name : Text
      , type : < gerrit | git | github | pagure >
      }
  , default =
    { base-url = None Text
    , github-app-name = None Text
    , github-label = None Text
    }
  , schema =
    { Type =
        { base-url : Optional Text
        , github-app-name : Optional Text
        , github-label : Optional Text
        , name : Text
        , type : < gerrit | git | github | pagure >
        }
    , default =
      { base-url = None Text
      , github-app-name = None Text
      , github-label = None Text
      }
    }
  }
, ConnectionType =
  { Type = < gerrit | git | github | pagure >
  , gerrit = < gerrit | git | github | pagure >.gerrit
  , git = < gerrit | git | github | pagure >.git
  , pagure = < gerrit | git | github | pagure >.pagure
  }
, GitACL =
  { Name =
      \(obj : { file : Text, groups : Optional (List Text), name : Text }) ->
        obj.name
  , Type = { file : Text, groups : Optional (List Text), name : Text }
  , default.groups = None (List Text)
  , schema =
    { Type = { file : Text, groups : Optional (List Text), name : Text }
    , default.groups = None (List Text)
    }
  }
, GitRepository =
  { Name =
      \ ( obj
        : { acl : Optional Text
          , branches : Optional (List Text)
          , default-branch : Optional Text
          , description : Optional Text
          , name : Text
          }
        ) ->
        obj.name
  , Type =
      { acl : Optional Text
      , branches : Optional (List Text)
      , default-branch : Optional Text
      , description : Optional Text
      , name : Text
      }
  , default =
    { acl = None Text
    , branches = None (List Text)
    , default-branch = None Text
    , description = None Text
    }
  , schema =
    { Type =
        { acl : Optional Text
        , branches : Optional (List Text)
        , default-branch : Optional Text
        , description : Optional Text
        , name : Text
        }
    , default =
      { acl = None Text
      , branches = None (List Text)
      , default-branch = None Text
      , description = None Text
      }
    }
  }
, Group =
  { Name =
      \ ( obj
        : { description : Optional Text
          , members : Optional (List Text)
          , name : Text
          }
        ) ->
        obj.name
  , Type =
      { description : Optional Text
      , members : Optional (List Text)
      , name : Text
      }
  , default = { description = None Text, members = None (List Text) }
  , schema =
    { Type =
        { description : Optional Text
        , members : Optional (List Text)
        , name : Text
        }
    , default = { description = None Text, members = None (List Text) }
    }
  }
, Project =
  { Name =
      \ ( project
        : { connection : Text
          , contacts : Optional (List Text)
          , description : Optional Text
          , documentation : Optional Text
          , issue-tracker-url : Optional Text
          , mailing-list : Optional (List Text)
          , name : Text
          , options : Optional (List Text)
          , review-dashboard : Optional Text
          , source-repositories :
              Optional
                ( List
                    < Inline :
                        List
                          { mapKey : Text
                          , mapValue :
                              { connection : Optional Text
                              , zuul/config-project : Optional Bool
                              , zuul/include : Optional (List Text)
                              }
                          }
                    | Name : Text
                    >
                )
          , tenant : Optional Text
          , website : Optional Text
          }
        ) ->
        project.name
  , Type =
      { connection : Text
      , contacts : Optional (List Text)
      , description : Optional Text
      , documentation : Optional Text
      , issue-tracker-url : Optional Text
      , mailing-list : Optional (List Text)
      , name : Text
      , options : Optional (List Text)
      , review-dashboard : Optional Text
      , source-repositories :
          Optional
            ( List
                < Inline :
                    List
                      { mapKey : Text
                      , mapValue :
                          { connection : Optional Text
                          , zuul/config-project : Optional Bool
                          , zuul/include : Optional (List Text)
                          }
                      }
                | Name : Text
                >
            )
      , tenant : Optional Text
      , website : Optional Text
      }
  , default =
    { contacts = None (List Text)
    , description = None Text
    , documentation = None Text
    , issue-tracker-url = None Text
    , mailing-list = None (List Text)
    , options = None (List Text)
    , review-dashboard = None Text
    , source-repositories =
        None
          ( List
              < Inline :
                  List
                    { mapKey : Text
                    , mapValue :
                        { connection : Optional Text
                        , zuul/config-project : Optional Bool
                        , zuul/include : Optional (List Text)
                        }
                    }
              | Name : Text
              >
          )
    , tenant = None Text
    , website = None Text
    }
  , schema =
    { Type =
        { connection : Text
        , contacts : Optional (List Text)
        , description : Optional Text
        , documentation : Optional Text
        , issue-tracker-url : Optional Text
        , mailing-list : Optional (List Text)
        , name : Text
        , options : Optional (List Text)
        , review-dashboard : Optional Text
        , source-repositories :
            Optional
              ( List
                  < Inline :
                      List
                        { mapKey : Text
                        , mapValue :
                            { connection : Optional Text
                            , zuul/config-project : Optional Bool
                            , zuul/include : Optional (List Text)
                            }
                        }
                  | Name : Text
                  >
              )
        , tenant : Optional Text
        , website : Optional Text
        }
    , default =
      { contacts = None (List Text)
      , description = None Text
      , documentation = None Text
      , issue-tracker-url = None Text
      , mailing-list = None (List Text)
      , options = None (List Text)
      , review-dashboard = None Text
      , source-repositories =
          None
            ( List
                < Inline :
                    List
                      { mapKey : Text
                      , mapValue :
                          { connection : Optional Text
                          , zuul/config-project : Optional Bool
                          , zuul/include : Optional (List Text)
                          }
                      }
                | Name : Text
                >
            )
      , tenant = None Text
      , website = None Text
      }
    }
  , sourceRepository =
    { SourceRepositoryOptionsType =
        List
          { mapKey : Text
          , mapValue :
              < BoolValue : Bool | ListValue : List Text | TextValue : Text >
          }
    , SourceRepositoryOptionsValue =
        < BoolValue : Bool | ListValue : List Text | TextValue : Text >
    , SourceRepositoryType =
        < ProjectName : Text
        | ProjectNameWithOption :
            List
              { mapKey : Text
              , mapValue :
                  List
                    { mapKey : Text
                    , mapValue :
                        < BoolValue : Bool
                        | ListValue : List Text
                        | TextValue : Text
                        >
                    }
              }
        >
    }
  }
, Resources =
  { Type =
      { acls : List { file : Text, groups : Optional (List Text), name : Text }
      , connections :
          List
            { base-url : Optional Text
            , github-app-name : Optional Text
            , github-label : Optional Text
            , name : Text
            , type : < gerrit | git | github | pagure >
            }
      , groups :
          List
            { description : Optional Text
            , members : Optional (List Text)
            , name : Text
            }
      , projects :
          List
            { connection : Text
            , contacts : Optional (List Text)
            , description : Optional Text
            , documentation : Optional Text
            , issue-tracker-url : Optional Text
            , mailing-list : Optional (List Text)
            , name : Text
            , options : Optional (List Text)
            , review-dashboard : Optional Text
            , source-repositories :
                Optional
                  ( List
                      < Inline :
                          List
                            { mapKey : Text
                            , mapValue :
                                { connection : Optional Text
                                , zuul/config-project : Optional Bool
                                , zuul/include : Optional (List Text)
                                }
                            }
                      | Name : Text
                      >
                  )
            , tenant : Optional Text
            , website : Optional Text
            }
      , repos :
          List
            { acl : Optional Text
            , branches : Optional (List Text)
            , default-branch : Optional Text
            , description : Optional Text
            , name : Text
            }
      , tenants :
          List
            { default-connection : Optional Text
            , description : Optional Text
            , name : Text
            , tenant-options :
                Optional
                  { zuul/max-job-timeout : Optional Natural
                  , zuul/report-build-page : Optional Bool
                  , zuul/web-url : Optional Text
                  }
            , url : Text
            }
      }
  , default = {=}
  , renderManagesf =
      \ ( resources
        : { acls :
              List { file : Text, groups : Optional (List Text), name : Text }
          , connections :
              List
                { base-url : Optional Text
                , github-app-name : Optional Text
                , github-label : Optional Text
                , name : Text
                , type : < gerrit | git | github | pagure >
                }
          , groups :
              List
                { description : Optional Text
                , members : Optional (List Text)
                , name : Text
                }
          , projects :
              List
                { connection : Text
                , contacts : Optional (List Text)
                , description : Optional Text
                , documentation : Optional Text
                , issue-tracker-url : Optional Text
                , mailing-list : Optional (List Text)
                , name : Text
                , options : Optional (List Text)
                , review-dashboard : Optional Text
                , source-repositories :
                    Optional
                      ( List
                          < Inline :
                              List
                                { mapKey : Text
                                , mapValue :
                                    { connection : Optional Text
                                    , zuul/config-project : Optional Bool
                                    , zuul/include : Optional (List Text)
                                    }
                                }
                          | Name : Text
                          >
                      )
                , tenant : Optional Text
                , website : Optional Text
                }
          , repos :
              List
                { acl : Optional Text
                , branches : Optional (List Text)
                , default-branch : Optional Text
                , description : Optional Text
                , name : Text
                }
          , tenants :
              List
                { default-connection : Optional Text
                , description : Optional Text
                , name : Text
                , tenant-options :
                    Optional
                      { zuul/max-job-timeout : Optional Natural
                      , zuul/report-build-page : Optional Bool
                      , zuul/web-url : Optional Text
                      }
                , url : Text
                }
          }
        ) ->
        { resources =
          { acls =
              List/fold
                { file : Text, groups : Optional (List Text), name : Text }
                resources.acls
                ( List
                    { mapKey : Text
                    , mapValue :
                        { file : Text
                        , groups : Optional (List Text)
                        , name : Text
                        }
                    }
                )
                ( \ ( _
                    : { file : Text
                      , groups : Optional (List Text)
                      , name : Text
                      }
                    ) ->
                  \ ( _
                    : List
                        { mapKey : Text
                        , mapValue :
                            { file : Text
                            , groups : Optional (List Text)
                            , name : Text
                            }
                        }
                    ) ->
                    [ { mapKey = _@1.name, mapValue = _@1 } ] # _
                )
                ( [] : List
                         { mapKey : Text
                         , mapValue :
                             { file : Text
                             , groups : Optional (List Text)
                             , name : Text
                             }
                         }
                )
          , connections =
              List/fold
                { base-url : Optional Text
                , github-app-name : Optional Text
                , github-label : Optional Text
                , name : Text
                , type : < gerrit | git | github | pagure >
                }
                resources.connections
                ( List
                    { mapKey : Text
                    , mapValue :
                        { base-url : Optional Text
                        , github-app-name : Optional Text
                        , github-label : Optional Text
                        , name : Text
                        , type : < gerrit | git | github | pagure >
                        }
                    }
                )
                ( \ ( _
                    : { base-url : Optional Text
                      , github-app-name : Optional Text
                      , github-label : Optional Text
                      , name : Text
                      , type : < gerrit | git | github | pagure >
                      }
                    ) ->
                  \ ( _
                    : List
                        { mapKey : Text
                        , mapValue :
                            { base-url : Optional Text
                            , github-app-name : Optional Text
                            , github-label : Optional Text
                            , name : Text
                            , type : < gerrit | git | github | pagure >
                            }
                        }
                    ) ->
                    [ { mapKey = _@1.name, mapValue = _@1 } ] # _
                )
                ( [] : List
                         { mapKey : Text
                         , mapValue :
                             { base-url : Optional Text
                             , github-app-name : Optional Text
                             , github-label : Optional Text
                             , name : Text
                             , type : < gerrit | git | github | pagure >
                             }
                         }
                )
          , groups =
              List/fold
                { description : Optional Text
                , members : Optional (List Text)
                , name : Text
                }
                resources.groups
                ( List
                    { mapKey : Text
                    , mapValue :
                        { description : Optional Text
                        , members : Optional (List Text)
                        , name : Text
                        }
                    }
                )
                ( \ ( _
                    : { description : Optional Text
                      , members : Optional (List Text)
                      , name : Text
                      }
                    ) ->
                  \ ( _
                    : List
                        { mapKey : Text
                        , mapValue :
                            { description : Optional Text
                            , members : Optional (List Text)
                            , name : Text
                            }
                        }
                    ) ->
                    [ { mapKey = _@1.name, mapValue = _@1 } ] # _
                )
                ( [] : List
                         { mapKey : Text
                         , mapValue :
                             { description : Optional Text
                             , members : Optional (List Text)
                             , name : Text
                             }
                         }
                )
          , projects =
              List/fold
                { connection : Text
                , contacts : Optional (List Text)
                , description : Optional Text
                , documentation : Optional Text
                , issue-tracker-url : Optional Text
                , mailing-list : Optional (List Text)
                , name : Text
                , options : Optional (List Text)
                , review-dashboard : Optional Text
                , source-repositories :
                    Optional
                      ( List
                          < Inline :
                              List
                                { mapKey : Text
                                , mapValue :
                                    { connection : Optional Text
                                    , zuul/config-project : Optional Bool
                                    , zuul/include : Optional (List Text)
                                    }
                                }
                          | Name : Text
                          >
                      )
                , tenant : Optional Text
                , website : Optional Text
                }
                resources.projects
                ( List
                    { mapKey : Text
                    , mapValue :
                        { connection : Text
                        , contacts : Optional (List Text)
                        , description : Optional Text
                        , documentation : Optional Text
                        , issue-tracker-url : Optional Text
                        , mailing-list : Optional (List Text)
                        , name : Text
                        , options : Optional (List Text)
                        , review-dashboard : Optional Text
                        , source-repositories :
                            Optional
                              ( List
                                  < Inline :
                                      List
                                        { mapKey : Text
                                        , mapValue :
                                            { connection : Optional Text
                                            , zuul/config-project :
                                                Optional Bool
                                            , zuul/include :
                                                Optional (List Text)
                                            }
                                        }
                                  | Name : Text
                                  >
                              )
                        , tenant : Optional Text
                        , website : Optional Text
                        }
                    }
                )
                ( \ ( _
                    : { connection : Text
                      , contacts : Optional (List Text)
                      , description : Optional Text
                      , documentation : Optional Text
                      , issue-tracker-url : Optional Text
                      , mailing-list : Optional (List Text)
                      , name : Text
                      , options : Optional (List Text)
                      , review-dashboard : Optional Text
                      , source-repositories :
                          Optional
                            ( List
                                < Inline :
                                    List
                                      { mapKey : Text
                                      , mapValue :
                                          { connection : Optional Text
                                          , zuul/config-project : Optional Bool
                                          , zuul/include : Optional (List Text)
                                          }
                                      }
                                | Name : Text
                                >
                            )
                      , tenant : Optional Text
                      , website : Optional Text
                      }
                    ) ->
                  \ ( _
                    : List
                        { mapKey : Text
                        , mapValue :
                            { connection : Text
                            , contacts : Optional (List Text)
                            , description : Optional Text
                            , documentation : Optional Text
                            , issue-tracker-url : Optional Text
                            , mailing-list : Optional (List Text)
                            , name : Text
                            , options : Optional (List Text)
                            , review-dashboard : Optional Text
                            , source-repositories :
                                Optional
                                  ( List
                                      < Inline :
                                          List
                                            { mapKey : Text
                                            , mapValue :
                                                { connection : Optional Text
                                                , zuul/config-project :
                                                    Optional Bool
                                                , zuul/include :
                                                    Optional (List Text)
                                                }
                                            }
                                      | Name : Text
                                      >
                                  )
                            , tenant : Optional Text
                            , website : Optional Text
                            }
                        }
                    ) ->
                    [ { mapKey = _@1.name, mapValue = _@1 } ] # _
                )
                ( [] : List
                         { mapKey : Text
                         , mapValue :
                             { connection : Text
                             , contacts : Optional (List Text)
                             , description : Optional Text
                             , documentation : Optional Text
                             , issue-tracker-url : Optional Text
                             , mailing-list : Optional (List Text)
                             , name : Text
                             , options : Optional (List Text)
                             , review-dashboard : Optional Text
                             , source-repositories :
                                 Optional
                                   ( List
                                       < Inline :
                                           List
                                             { mapKey : Text
                                             , mapValue :
                                                 { connection : Optional Text
                                                 , zuul/config-project :
                                                     Optional Bool
                                                 , zuul/include :
                                                     Optional (List Text)
                                                 }
                                             }
                                       | Name : Text
                                       >
                                   )
                             , tenant : Optional Text
                             , website : Optional Text
                             }
                         }
                )
          , repos =
              List/fold
                { acl : Optional Text
                , branches : Optional (List Text)
                , default-branch : Optional Text
                , description : Optional Text
                , name : Text
                }
                resources.repos
                ( List
                    { mapKey : Text
                    , mapValue :
                        { acl : Optional Text
                        , branches : Optional (List Text)
                        , default-branch : Optional Text
                        , description : Optional Text
                        , name : Text
                        }
                    }
                )
                ( \ ( _
                    : { acl : Optional Text
                      , branches : Optional (List Text)
                      , default-branch : Optional Text
                      , description : Optional Text
                      , name : Text
                      }
                    ) ->
                  \ ( _
                    : List
                        { mapKey : Text
                        , mapValue :
                            { acl : Optional Text
                            , branches : Optional (List Text)
                            , default-branch : Optional Text
                            , description : Optional Text
                            , name : Text
                            }
                        }
                    ) ->
                    [ { mapKey = _@1.name, mapValue = _@1 } ] # _
                )
                ( [] : List
                         { mapKey : Text
                         , mapValue :
                             { acl : Optional Text
                             , branches : Optional (List Text)
                             , default-branch : Optional Text
                             , description : Optional Text
                             , name : Text
                             }
                         }
                )
          , tenants =
              List/fold
                { default-connection : Optional Text
                , description : Optional Text
                , name : Text
                , tenant-options :
                    Optional
                      { zuul/max-job-timeout : Optional Natural
                      , zuul/report-build-page : Optional Bool
                      , zuul/web-url : Optional Text
                      }
                , url : Text
                }
                resources.tenants
                ( List
                    { mapKey : Text
                    , mapValue :
                        { default-connection : Optional Text
                        , description : Optional Text
                        , name : Text
                        , tenant-options :
                            Optional
                              { zuul/max-job-timeout : Optional Natural
                              , zuul/report-build-page : Optional Bool
                              , zuul/web-url : Optional Text
                              }
                        , url : Text
                        }
                    }
                )
                ( \ ( _
                    : { default-connection : Optional Text
                      , description : Optional Text
                      , name : Text
                      , tenant-options :
                          Optional
                            { zuul/max-job-timeout : Optional Natural
                            , zuul/report-build-page : Optional Bool
                            , zuul/web-url : Optional Text
                            }
                      , url : Text
                      }
                    ) ->
                  \ ( _
                    : List
                        { mapKey : Text
                        , mapValue :
                            { default-connection : Optional Text
                            , description : Optional Text
                            , name : Text
                            , tenant-options :
                                Optional
                                  { zuul/max-job-timeout : Optional Natural
                                  , zuul/report-build-page : Optional Bool
                                  , zuul/web-url : Optional Text
                                  }
                            , url : Text
                            }
                        }
                    ) ->
                    [ { mapKey = _@1.name, mapValue = _@1 } ] # _
                )
                ( [] : List
                         { mapKey : Text
                         , mapValue :
                             { default-connection : Optional Text
                             , description : Optional Text
                             , name : Text
                             , tenant-options :
                                 Optional
                                   { zuul/max-job-timeout : Optional Natural
                                   , zuul/report-build-page : Optional Bool
                                   , zuul/web-url : Optional Text
                                   }
                             , url : Text
                             }
                         }
                )
          }
        }
  , renderZuul =
      \ ( resources
        : { acls :
              List { file : Text, groups : Optional (List Text), name : Text }
          , connections :
              List
                { base-url : Optional Text
                , github-app-name : Optional Text
                , github-label : Optional Text
                , name : Text
                , type : < gerrit | git | github | pagure >
                }
          , groups :
              List
                { description : Optional Text
                , members : Optional (List Text)
                , name : Text
                }
          , projects :
              List
                { connection : Text
                , contacts : Optional (List Text)
                , description : Optional Text
                , documentation : Optional Text
                , issue-tracker-url : Optional Text
                , mailing-list : Optional (List Text)
                , name : Text
                , options : Optional (List Text)
                , review-dashboard : Optional Text
                , source-repositories :
                    Optional
                      ( List
                          < Inline :
                              List
                                { mapKey : Text
                                , mapValue :
                                    { connection : Optional Text
                                    , zuul/config-project : Optional Bool
                                    , zuul/include : Optional (List Text)
                                    }
                                }
                          | Name : Text
                          >
                      )
                , tenant : Optional Text
                , website : Optional Text
                }
          , repos :
              List
                { acl : Optional Text
                , branches : Optional (List Text)
                , default-branch : Optional Text
                , description : Optional Text
                , name : Text
                }
          , tenants :
              List
                { default-connection : Optional Text
                , description : Optional Text
                , name : Text
                , tenant-options :
                    Optional
                      { zuul/max-job-timeout : Optional Natural
                      , zuul/report-build-page : Optional Bool
                      , zuul/web-url : Optional Text
                      }
                , url : Text
                }
          }
        ) ->
        List/fold
          { default-connection : Optional Text
          , description : Optional Text
          , name : Text
          , tenant-options :
              Optional
                { zuul/max-job-timeout : Optional Natural
                , zuul/report-build-page : Optional Bool
                , zuul/web-url : Optional Text
                }
          , url : Text
          }
          resources.tenants
          (List { tenant : { name : Text, report-build-page : Optional Bool } })
          ( \ ( _
              : { default-connection : Optional Text
                , description : Optional Text
                , name : Text
                , tenant-options :
                    Optional
                      { zuul/max-job-timeout : Optional Natural
                      , zuul/report-build-page : Optional Bool
                      , zuul/web-url : Optional Text
                      }
                , url : Text
                }
              ) ->
            \ ( _
              : List
                  { tenant : { name : Text, report-build-page : Optional Bool }
                  }
              ) ->
                [ { tenant =
                    { name = _@1.name
                    , report-build-page =
                        ( merge
                            { None =
                              { zuul/max-job-timeout = None Natural
                              , zuul/report-build-page = None Bool
                              , zuul/web-url = None Text
                              }
                            , Some =
                                \ ( some
                                  : { zuul/max-job-timeout : Optional Natural
                                    , zuul/report-build-page : Optional Bool
                                    , zuul/web-url : Optional Text
                                    }
                                  ) ->
                                  some
                            }
                            _@1.tenant-options
                        ).zuul/report-build-page
                    }
                  }
                ]
              # _
          )
          ( [] : List
                   { tenant : { name : Text, report-build-page : Optional Bool }
                   }
          )
  }
, SourceRepository =
  { Name =
      < Inline :
          List
            { mapKey : Text
            , mapValue :
                { connection : Optional Text
                , zuul/config-project : Optional Bool
                , zuul/include : Optional (List Text)
                }
            }
      | Name : Text
      >.Name
  , Type =
      { connection : Optional Text
      , zuul/config-project : Optional Bool
      , zuul/include : Optional (List Text)
      }
  , Union =
      < Inline :
          List
            { mapKey : Text
            , mapValue :
                { connection : Optional Text
                , zuul/config-project : Optional Bool
                , zuul/include : Optional (List Text)
                }
            }
      | Name : Text
      >
  , WithOptions =
      \ ( sr
        : { connection : Optional Text
          , zuul/config-project : Optional Bool
          , zuul/include : Optional (List Text)
          }
        ) ->
      \(name : Text) ->
        < Inline :
            List
              { mapKey : Text
              , mapValue :
                  { connection : Optional Text
                  , zuul/config-project : Optional Bool
                  , zuul/include : Optional (List Text)
                  }
              }
        | Name : Text
        >.Inline
          [ { mapKey = name, mapValue = sr } ]
  , default =
    { connection = None Text
    , zuul/config-project = None Bool
    , zuul/include = None (List Text)
    }
  , schema.Type
    =
      { connection : Optional Text
      , zuul/config-project : Optional Bool
      , zuul/include : Optional (List Text)
      }
  }
, Tenant =
  { Name =
      \ ( tenant
        : { default-connection : Optional Text
          , description : Optional Text
          , name : Text
          , tenant-options :
              Optional
                { zuul/max-job-timeout : Optional Natural
                , zuul/report-build-page : Optional Bool
                , zuul/web-url : Optional Text
                }
          , url : Text
          }
        ) ->
        tenant.name
  , Type =
      { default-connection : Optional Text
      , description : Optional Text
      , name : Text
      , tenant-options :
          Optional
            { zuul/max-job-timeout : Optional Natural
            , zuul/report-build-page : Optional Bool
            , zuul/web-url : Optional Text
            }
      , url : Text
      }
  , default =
    { default-connection = None Text
    , description = None Text
    , tenant-options =
        None
          { zuul/max-job-timeout : Optional Natural
          , zuul/report-build-page : Optional Bool
          , zuul/web-url : Optional Text
          }
    }
  , getOptions =
      \ ( tenant
        : { default-connection : Optional Text
          , description : Optional Text
          , name : Text
          , tenant-options :
              Optional
                { zuul/max-job-timeout : Optional Natural
                , zuul/report-build-page : Optional Bool
                , zuul/web-url : Optional Text
                }
          , url : Text
          }
        ) ->
        merge
          { None =
            { zuul/max-job-timeout = None Natural
            , zuul/report-build-page = None Bool
            , zuul/web-url = None Text
            }
          , Some =
              \ ( some
                : { zuul/max-job-timeout : Optional Natural
                  , zuul/report-build-page : Optional Bool
                  , zuul/web-url : Optional Text
                  }
                ) ->
                some
          }
          tenant.tenant-options
  }
, TenantOptions =
  { Type =
      { zuul/max-job-timeout : Optional Natural
      , zuul/report-build-page : Optional Bool
      , zuul/web-url : Optional Text
      }
  , default =
    { zuul/max-job-timeout = None Natural
    , zuul/report-build-page = None Bool
    , zuul/web-url = None Text
    }
  }
}
