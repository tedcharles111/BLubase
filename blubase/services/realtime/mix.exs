defmodule Realtime.MixProject do
  use Mix.Project
  def project do
    [
      app: :realtime,
      version: "0.1.0",
      elixir: "~> 1.15",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end
  defp deps do
    [
      {:plug_cowboy, "~> 2.6"},
      {:jason, "~> 1.4"},
      {:redix, "~> 1.2"}
    ]
  end
end
