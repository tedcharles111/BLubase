defmodule Realtime.ChangeListener do
  use GenServer
  def start_link(_) do
    GenServer.start_link(__MODULE__, %{}, name: __MODULE__)
  end
  def init(_) do
    Redix.command(:redix, ["PSUBSCRIBE", "realtime:*"])
    {:ok, %{}}
  end
  def handle_info({:redix_pubsub, :pmessage, pattern, channel, message}, state) do
    # Forward to all connected WebSocket clients (simplified)
    {:noreply, state}
  end
end
