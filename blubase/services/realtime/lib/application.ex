defmodule Realtime.Application do
  use Application
  def start(_type, _args) do
    children = [
      {Redix, name: :redix, host: "redis", port: 6379},
      {Plug.Cowboy, scheme: :http, plug: Realtime.Router, options: [port: 4000]}
    ]
    opts = [strategy: :one_for_one, name: Realtime.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
