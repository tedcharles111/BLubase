defmodule Realtime.Application do
  use Application

  def start(_type, _args) do
    children = [
      {Phoenix.PubSub, name: Realtime.PubSub},
      RealtimeWeb.Endpoint,
      {Redix, name: :redix, host: "redis", port: 6379},
      Realtime.ChangeListener
    ]
    opts = [strategy: :one_for_one, name: Realtime.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
