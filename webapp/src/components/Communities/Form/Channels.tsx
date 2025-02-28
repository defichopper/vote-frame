import { Box, FormControl, FormErrorMessage, FormLabel, Heading, Image, Text } from '@chakra-ui/react'
import { AsyncSelect, chakraComponents as components, GroupBase, OptionProps } from 'chakra-react-select'
import { useState } from 'react'
import { Controller, useFormContext } from 'react-hook-form'
import { useAuth } from '~components/Auth/useAuth'
import { fetchChannelQuery } from '~queries/channels'

// CustomOption Component
const CustomOption = (props: OptionProps<any, false, GroupBase<any>>) => {
  return (
    <components.Option {...props}>
      <Box display='flex' alignItems='center'>
        <Image
          src={props.data.image} // Image URL from the option data
          borderRadius='full' // Makes the image circular
          boxSize='20px' // Sets the size of the image
          objectFit='cover' // Ensures the image covers the area properly
          mr='8px' // Right margin for spacing
          alt={props.data.label} // Alt text for accessibility
        />
        <Text>{props.data.label}</Text>
      </Box>
    </components.Option>
  )
}

export type ChannelsFormValues = {
  channels: { label: string; value: string; image: string }[]
}

export const Channels = () => {
  const {
    formState: { errors },
    setError,
    clearErrors,
  } = useFormContext<ChannelsFormValues>()
  const [loading, setLoading] = useState<boolean>(false)
  const { bfetch } = useAuth()

  return (
    <FormControl display='flex' flexDir='column' gap={4} isInvalid={!!errors.channels}>
      <Heading as={FormLabel} size='sm'>
        Add Farcaster Channels
      </Heading>
      <Text>Add the farcaster channels used by your community</Text>
      <Controller
        name='channels'
        render={({ field }) => (
          <AsyncSelect
            id='channels'
            size='sm'
            // @ts-expect-error bad typing definition (allows false or undefined but not true, which is false)
            isMulti
            isLoading={loading}
            noOptionsMessage={() => 'No channels found'}
            placeholder='Search and add channels'
            {...field}
            components={{ Option: CustomOption }}
            loadOptions={async (inputValue) => {
              try {
                clearErrors('channels')
                setLoading(true)
                return (await fetchChannelQuery(bfetch)(inputValue)).map((channel) => ({
                  label: channel.name,
                  image: channel.image,
                  value: channel.id,
                }))
              } catch (e) {
                console.error('Could not fetch channels:', e)
                if (e instanceof Error) {
                  setError('channels', { message: e.message })
                }
                return []
              } finally {
                setLoading(false)
              }
            }}
          />
        )}
      />
      <FormErrorMessage>{errors.channels?.message?.toString()}</FormErrorMessage>
    </FormControl>
  )
}
