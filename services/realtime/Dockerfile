FROM elixir:1.15-alpine AS builder
RUN mix local.hex --force && mix local.rebar --force
WORKDIR /app
COPY mix.exs .
RUN mix deps.get
COPY . .
RUN mix compile

FROM elixir:1.15-alpine
WORKDIR /app
COPY --from=builder /app/_build /app/_build
COPY --from=builder /app/deps /app/deps
COPY --from=builder /app/mix.exs /app/
COPY --from=builder /app/lib /app/lib
CMD ["elixir", "-S", "mix", "run", "--no-halt"]
