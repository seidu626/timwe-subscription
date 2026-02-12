import { NextRequest, NextResponse } from 'next/server'

const ACQUISITION_API_URL = process.env.ACQUISITION_API_URL || 'http://localhost:8084'

export async function POST(
  request: NextRequest,
  { params }: { params: { id: string } }
) {
  const { id } = params

  try {
    const body = await request.json()

    const response = await fetch(
      `${ACQUISITION_API_URL}/v1/acquisition/transactions/${id}/confirm`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      }
    )

    const data = await response.json()

    if (!response.ok) {
      return NextResponse.json(data, { status: response.status })
    }

    return NextResponse.json(data)
  } catch (error) {
    console.error('Failed to confirm transaction:', error)
    return NextResponse.json(
      { error: 'Failed to confirm transaction' },
      { status: 500 }
    )
  }
}
